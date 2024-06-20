package shortage_detector

// PodRanker is an interface for filtering pods
type ShortageDetector interface {
	ShouldEvict() (bool, error)
}
