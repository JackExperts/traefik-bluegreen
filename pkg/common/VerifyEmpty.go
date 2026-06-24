package common

func VerifyEmpty(value string, defaultValue string) string {
	if value == "" {
		return defaultValue
	}
	return value
}
