package types

type FlannelNetConf struct {
	Network   string             `json:"Network"`
	SubnetLen int                `json:"SubnetLen"`
	SubnetMin string             `json:"SubnetMin"`
	SubnetMax string             `json:"SubnetMax"`
	Backend   FlannelBackendConf `json:"Backend"`
}

type FlannelBackendConf struct {
	Type string `json:"Type"`
	Port int    `json:"Port"`
}
