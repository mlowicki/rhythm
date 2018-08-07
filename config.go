package main

type Config struct {
	GitLab          ConfigGitLab
	API             ConfigAPI
	Vault           ConfigVault
	Storage         string
	ZooKeeper       ConfigZooKeeper
	FailoverTimeout float64
	Verbose         bool
}

type ConfigAPI struct {
	Address string
}

type ConfigVault struct {
	Token   string
	Address string
	Timeout int
}

type ConfigGitLab struct {
	BaseURL string
}

type ConfigZooKeeper struct {
	BasePath string
}
