package types

type SecretValue struct {
	Name  string
	Value string
}

type GetSecretOpt struct {
	Namespace string
	Name      string
	Key       string
}
