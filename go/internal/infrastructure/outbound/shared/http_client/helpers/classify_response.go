package helpers

// =============================================================================
// CLASSIFY RESPONSE
// =============================================================================

// Class is what a response (or the lack of one) means for the delivery of a batch.
type Class string

const (
	// ClassSuccess means the destination accepted the batch.
	ClassSuccess Class = "success"
	// ClassTransient means that trying again may work: a timeout, a network error, 408, 429, 5xx or an invalid status.
	ClassTransient Class = "transient"
	// ClassPermanent means that trying again with the same batch is useless: any other 3xx or 4xx.
	ClassPermanent Class = "permanent"
)

// ClassifyStatus returns the class of an HTTP status. 2xx is success; 408, 429 and 5xx are transient; every other 3xx
// and 4xx is permanent (a redirect is never followed); a status that is not a valid HTTP status (below 200 included:
// an informational response is not a final answer) is transient, an invalid response that is retried with a limit.
func ClassifyStatus(status int) Class {
	switch {
	case status >= 200 && status <= 299:
		return ClassSuccess
	case status == 408 || status == 429:
		return ClassTransient
	case status >= 500 && status <= 599:
		return ClassTransient
	case status >= 300 && status <= 499:
		return ClassPermanent
	default:
		return ClassTransient
	}
}
