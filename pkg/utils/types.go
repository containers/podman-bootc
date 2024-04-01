package utils

type MachineInspect struct {
	ConnectionInfo ConnectionInfo `json:"ConnectionInfo"`
	SSHConfig      SSHConfig      `json:"SSHConfig"`
	Rootful        bool           `json:"Rootful"`
}

type PodmanSocket struct {
	Path string `json:"Path"`
}

type ConnectionInfo struct {
	PodmanSocket PodmanSocket `json:"PodmanSocket"`
}

type SSHConfig struct {
	IdentityPath string `json:"IdentityPath"`
}

type MachineList struct {
	Name    string `json:"Name"`
	Running bool   `json:"Running"`
	Default bool   `json:"Default"`
}
