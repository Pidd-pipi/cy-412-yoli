package constants

const (
	VisitorPassStatusPending  = "pending"
	VisitorPassStatusApproved = "approved"
	VisitorPassStatusRevoked  = "revoked"
	VisitorPassStatusExpired  = "expired"
)

var ValidVisitorPassStatuses = map[string]bool{VisitorPassStatusPending: true, VisitorPassStatusApproved: true, VisitorPassStatusRevoked: true, VisitorPassStatusExpired: true}
