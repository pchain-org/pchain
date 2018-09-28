package params

var GenCfg = GeneralConfig{PerfTest:false}

//configuations in this structure is read-only, it gives a way to put/get general settings
type GeneralConfig struct {

	// Whether doing performance test, will remove some limitations and cause system more frigile
	PerfTest bool     `json:"perfTest,omitempty"`
}

