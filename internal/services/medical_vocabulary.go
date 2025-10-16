package services

import (
	"encoding/json"
	"strings"
	"sync"
)

// MedicalVocabularyService handles medical term recognition and validation
type MedicalVocabularyService struct {
	medicalTerms map[string]MedicalTermInfo
	mutex        sync.RWMutex
}

// MedicalTermInfo contains information about a medical term
type MedicalTermInfo struct {
	Term        string   `json:"term"`
	Category    string   `json:"category"`
	Synonyms    []string `json:"synonyms"`
	Confidence  float64  `json:"confidence"`
	Description string   `json:"description"`
}

// MedicalTermCategory represents different categories of medical terms
type MedicalTermCategory string

const (
	CategoryVitals       MedicalTermCategory = "vitals"
	CategoryMedications  MedicalTermCategory = "medications"
	CategorySymptoms     MedicalTermCategory = "symptoms"
	CategoryDiagnosis    MedicalTermCategory = "diagnosis"
	CategoryProcedures   MedicalTermCategory = "procedures"
	CategoryAnatomy      MedicalTermCategory = "anatomy"
	CategoryUnits        MedicalTermCategory = "units"
)

// NewMedicalVocabularyService creates a new medical vocabulary service
func NewMedicalVocabularyService() *MedicalVocabularyService {
	service := &MedicalVocabularyService{
		medicalTerms: make(map[string]MedicalTermInfo),
	}
	service.initializeMedicalTerms()
	return service
}

// initializeMedicalTerms populates the medical vocabulary database
func (mvs *MedicalVocabularyService) initializeMedicalTerms() {
	mvs.mutex.Lock()
	defer mvs.mutex.Unlock()

	// Vitals terms
	vitalsTerms := map[string]MedicalTermInfo{
		"blood pressure": {
			Term:        "blood pressure",
			Category:    string(CategoryVitals),
			Synonyms:    []string{"bp", "arterial pressure", "systemic pressure"},
			Confidence:  0.95,
			Description: "The pressure of blood against the walls of arteries",
		},
		"systolic": {
			Term:        "systolic",
			Category:    string(CategoryVitals),
			Synonyms:    []string{"systolic pressure", "top number"},
			Confidence:  0.9,
			Description: "The pressure when the heart beats",
		},
		"diastolic": {
			Term:        "diastolic",
			Category:    string(CategoryVitals),
			Synonyms:    []string{"diastolic pressure", "bottom number"},
			Confidence:  0.9,
			Description: "The pressure when the heart rests between beats",
		},
		"heart rate": {
			Term:        "heart rate",
			Category:    string(CategoryVitals),
			Synonyms:    []string{"pulse", "heartbeat", "bpm", "beats per minute"},
			Confidence:  0.95,
			Description: "Number of heartbeats per minute",
		},
		"temperature": {
			Term:        "temperature",
			Category:    string(CategoryVitals),
			Synonyms:    []string{"temp", "body temperature", "fever"},
			Confidence:  0.9,
			Description: "Body temperature measurement",
		},
		"respiratory rate": {
			Term:        "respiratory rate",
			Category:    string(CategoryVitals),
			Synonyms:    []string{"breathing rate", "respiration", "breaths per minute"},
			Confidence:  0.9,
			Description: "Number of breaths per minute",
		},
		"oxygen saturation": {
			Term:        "oxygen saturation",
			Category:    string(CategoryVitals),
			Synonyms:    []string{"spo2", "oxygen level", "o2 sat"},
			Confidence:  0.9,
			Description: "Percentage of oxygen in the blood",
		},
	}

	// Medication terms
	medicationTerms := map[string]MedicalTermInfo{
		"acetaminophen": {
			Term:        "acetaminophen",
			Category:    string(CategoryMedications),
			Synonyms:    []string{"tylenol", "paracetamol"},
			Confidence:  0.95,
			Description: "Pain reliever and fever reducer",
		},
		"ibuprofen": {
			Term:        "ibuprofen",
			Category:    string(CategoryMedications),
			Synonyms:    []string{"advil", "motrin"},
			Confidence:  0.95,
			Description: "Nonsteroidal anti-inflammatory drug",
		},
		"aspirin": {
			Term:        "aspirin",
			Category:    string(CategoryMedications),
			Synonyms:    []string{"asa", "acetylsalicylic acid"},
			Confidence:  0.95,
			Description: "Pain reliever and blood thinner",
		},
	}

	// Unit terms
	unitTerms := map[string]MedicalTermInfo{
		"millimeters of mercury": {
			Term:        "mmHg",
			Category:    string(CategoryUnits),
			Synonyms:    []string{"mmhg", "millimeters mercury"},
			Confidence:  0.9,
			Description: "Unit of pressure measurement",
		},
		"beats per minute": {
			Term:        "bpm",
			Category:    string(CategoryUnits),
			Synonyms:    []string{"beats/min", "per minute"},
			Confidence:  0.9,
			Description: "Unit for heart rate measurement",
		},
		"degrees fahrenheit": {
			Term:        "°F",
			Category:    string(CategoryUnits),
			Synonyms:    []string{"fahrenheit", "degrees f"},
			Confidence:  0.9,
			Description: "Temperature unit",
		},
		"degrees celsius": {
			Term:        "°C",
			Category:    string(CategoryUnits),
			Synonyms:    []string{"celsius", "degrees c"},
			Confidence:  0.9,
			Description: "Temperature unit",
		},
	}

	// Merge all terms
	for term, info := range vitalsTerms {
		mvs.medicalTerms[term] = info
	}
	for term, info := range medicationTerms {
		mvs.medicalTerms[term] = info
	}
	for term, info := range unitTerms {
		mvs.medicalTerms[term] = info
	}
}

// AnalyzeText analyzes text for medical terms and returns highlighted terms
func (mvs *MedicalVocabularyService) AnalyzeText(text string) []MedicalTerm {
	mvs.mutex.RLock()
	defer mvs.mutex.RUnlock()

	var medicalTerms []MedicalTerm
	words := strings.Fields(strings.ToLower(text))

	for i, word := range words {
		// Check single words
		if termInfo, exists := mvs.medicalTerms[word]; exists {
			medicalTerms = append(medicalTerms, MedicalTerm{
				Term:       word,
				Confidence: termInfo.Confidence,
				Start:      float64(i),
				End:        float64(i + 1),
				Category:   termInfo.Category,
			})
		}

		// Check multi-word terms (up to 3 words)
		for j := 1; j <= 2 && i+j < len(words); j++ {
			phrase := strings.Join(words[i:i+j+1], " ")
			if termInfo, exists := mvs.medicalTerms[phrase]; exists {
				medicalTerms = append(medicalTerms, MedicalTerm{
					Term:       phrase,
					Confidence: termInfo.Confidence,
					Start:      float64(i),
					End:        float64(i + j + 1),
					Category:   termInfo.Category,
				})
			}
		}
	}

	return medicalTerms
}

// GetTermInfo returns information about a specific medical term
func (mvs *MedicalVocabularyService) GetTermInfo(term string) (*MedicalTermInfo, bool) {
	mvs.mutex.RLock()
	defer mvs.mutex.RUnlock()

	termInfo, exists := mvs.medicalTerms[strings.ToLower(term)]
	return &termInfo, exists
}

// SuggestCorrections suggests corrections for low-confidence medical terms
func (mvs *MedicalVocabularyService) SuggestCorrections(term string) []string {
	mvs.mutex.RLock()
	defer mvs.mutex.RUnlock()

	var suggestions []string
	termLower := strings.ToLower(term)

	// Find terms with similar spelling
	for medicalTerm := range mvs.medicalTerms {
		if mvs.calculateSimilarity(termLower, medicalTerm) > 0.7 {
			suggestions = append(suggestions, medicalTerm)
		}
	}

	return suggestions
}

// calculateSimilarity calculates string similarity using Levenshtein distance
func (mvs *MedicalVocabularyService) calculateSimilarity(s1, s2 string) float64 {
	if s1 == s2 {
		return 1.0
	}

	len1, len2 := len(s1), len(s2)
	if len1 == 0 {
		return 0.0
	}
	if len2 == 0 {
		return 0.0
	}

	matrix := make([][]int, len1+1)
	for i := range matrix {
		matrix[i] = make([]int, len2+1)
	}

	for i := 0; i <= len1; i++ {
		matrix[i][0] = i
	}
	for j := 0; j <= len2; j++ {
		matrix[0][j] = j
	}

	for i := 1; i <= len1; i++ {
		for j := 1; j <= len2; j++ {
			cost := 0
			if s1[i-1] != s2[j-1] {
				cost = 1
			}
			matrix[i][j] = min(
				min(matrix[i-1][j]+1, matrix[i][j-1]+1), // deletion and insertion
				matrix[i-1][j-1]+cost, // substitution
			)
		}
	}

	maxLen := max(len1, len2)
	return 1.0 - float64(matrix[len1][len2])/float64(maxLen)
}

// NormalizeVitals normalizes vital signs text
func (mvs *MedicalVocabularyService) NormalizeVitals(text string) string {
	normalized := text

	// Normalize blood pressure
	normalized = strings.ReplaceAll(normalized, "one twenty over eighty", "120/80 mmHg")
	normalized = strings.ReplaceAll(normalized, "one twenty over ninety", "120/90 mmHg")
	normalized = strings.ReplaceAll(normalized, "one forty over ninety", "140/90 mmHg")

	// Normalize heart rate
	normalized = strings.ReplaceAll(normalized, "seventy two beats per minute", "72 bpm")
	normalized = strings.ReplaceAll(normalized, "eighty beats per minute", "80 bpm")

	// Normalize temperature
	normalized = strings.ReplaceAll(normalized, "ninety eight point six degrees", "98.6°F")
	normalized = strings.ReplaceAll(normalized, "thirty seven degrees celsius", "37°C")

	return normalized
}

// ExportMedicalTerms exports all medical terms as JSON
func (mvs *MedicalVocabularyService) ExportMedicalTerms() ([]byte, error) {
	mvs.mutex.RLock()
	defer mvs.mutex.RUnlock()

	return json.MarshalIndent(mvs.medicalTerms, "", "  ")
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// max returns the maximum of two integers
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
