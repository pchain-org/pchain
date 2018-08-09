package rpcclient

import (
	"io/ioutil"
	"net/http"
	"github.com/tendermint/go-rpc/server"
)

// Channel Client takes params as a map
type ChannelClient struct {
	client *http.Client
}

func NewChannelClient(cl *rpcserver.ChannelListener) *ChannelClient {
	client := makeChannelClient(cl)
	return &ChannelClient{
		client: client,
	}
}

func makeChannelClient(cl *rpcserver.ChannelListener) *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			Dial: cl.Dial,
		},
	}
}

func (c *ChannelClient) Call(method string, params map[string]interface{}, result interface{}) (interface{}, error) {
	values, err := argsToURLValues(params)
	if err != nil {
		return nil, err
	}
	// log.Info(Fmt("URI request to %v (%v): %v", c.address, method, values))
	resp, err := c.client.PostForm("http://localhost/"+method, values)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	responseBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return unmarshalResponseBytes(responseBytes, result)
}

func (c *ChannelClient) Get(method string) (resp *http.Response, err error) {

	resp, err = c.client.Get("http://localhost/"+method)
	return resp, err
}
