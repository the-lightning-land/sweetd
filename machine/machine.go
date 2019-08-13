package machine

type Machine interface {
	Start() error
	Stop() error
	TouchEvents() <-chan bool
	ToggleMotor(on bool)
	ToggleBuzzer(on bool)
	DiagnosticNoise()
}