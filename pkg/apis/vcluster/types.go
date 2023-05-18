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
	Enabled          bool   `yaml:"enabled"`
	PathType         string `yaml:"pathType"`
	ApiVersion       string `yaml:"apiVersion"`
	IngressClassName string `yaml:"ingressClassName"`
	Host             string `yaml:"host"`
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

// Available values for vcluster helm chart: https://artifacthub.io/packages/helm/loft/vcluster
type ValuesTemplate struct {
	Openshift Openshift `yaml:"openshift"`
	Sync      Sync      `yaml:"sync"`
}
