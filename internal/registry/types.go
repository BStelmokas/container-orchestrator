package registry

// ServiceInstance represents one concrete backend replica for a logical service.
type ServiceInstance struct {
	ServiceName string `json:"serviceName"`
	InstanceID string `json:"instanceID"`
	IP string `json:"ip"`
	Port int `json:"port"`
}
