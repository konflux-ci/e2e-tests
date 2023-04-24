package vcluster

type Openshift struct {
	Enable bool `yaml:"enable"`
}

type NetworkPolicies struct {
	Enabled bool `yaml:"enabled"`
}

type ServiceAccounts struct {
	Enabled bool `yaml:"enabled"`
}

type Services struct {
	SyncServiceSelector bool `yaml:"syncServiceSelector"`
}

type Ingresses struct {
	Enabled bool `yaml:"enabled"`
}

type Secrets struct {
	Enabled bool `yaml:"enabled"`
	All     bool `yaml:"all"`
}

type Sync struct {
	NetworkPolicies NetworkPolicies `yaml:"networkpolicies"`
	ServiceAccounts ServiceAccounts `yaml:"serviceaccounts"`
	Services        Services        `yaml:"services"`
	Ingresses       Ingresses       `yaml:"ingresses"`
	Secrets         Secrets         `yaml:"secrets"`
}

type ValuesTemplate struct {
	Openshift Openshift `yaml:"openshift"`
	Sync      Sync      `yaml:"sync"`
}
