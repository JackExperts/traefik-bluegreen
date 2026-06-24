package redis

import (
	"fmt"
	"strings"
)

func Serializer(command string, args ...string) string {
	var stringBuilder strings.Builder

	fmt.Fprintf(
		&stringBuilder, "*%d\r\n$%d\r\n%s\r\n",
		len(args)+1, len(command), command,
	) // command + quantidade de args

	for i := 0; i < len(args); i++ {
		fmt.Fprintf(
			&stringBuilder, "$%d\r\n%s\r\n",
			len(args[i]), args[i],
		)
	}

	return stringBuilder.String()

}
