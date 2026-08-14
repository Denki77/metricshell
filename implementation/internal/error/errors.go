package err

type ErrorMessage string

func (e ErrorMessage) Error() string {
	return string(e)
}

type bootstrapContainer struct {
	CommandRequired   ErrorMessage
	SeparatorRequired ErrorMessage
	UnknownOption     ErrorMessage
}

var Bootstrap = bootstrapContainer{
	CommandRequired:   "workload command after -- is required",
	SeparatorRequired: "workload separator -- is required",
	UnknownOption:     "unknown option",
}
