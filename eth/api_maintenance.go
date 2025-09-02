package eth

type MaintenanceAPI struct {
	eth *Ethereum
}

func NewMaintenanceAPI(eth *Ethereum) *MaintenanceAPI {
	return &MaintenanceAPI{eth: eth}
}

func (api *MaintenanceAPI) StopAPIFeed() (bool, error) {
	return api.eth.tool.APIFeedSwitch(false), nil
}

func (api *MaintenanceAPI) StartAPIFeed() (bool, error) {
	return api.eth.tool.APIFeedSwitch(true), nil
}
