package testkit

func PanicValue(call func()) (value any) {
	defer func() { value = recover() }()
	call()
	return nil
}
