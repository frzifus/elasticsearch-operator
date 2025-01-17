package helpers

import (
	"encoding/json"

	"github.com/ViaQ/logerr/kverrors"
	"github.com/ViaQ/logerr/log"
	"github.com/go-logr/logr"
	"github.com/openshift/elasticsearch-operator/internal/elasticsearch/esclient"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func NewFakeElasticsearchChatter(responses map[string]FakeElasticsearchResponses) *FakeElasticsearchChatter {
	// Copy responses into chatter to unshare between multiple instances
	fres := make(map[string]FakeElasticsearchResponses)
	for key, res := range responses {
		fres[key] = res
	}

	return &FakeElasticsearchChatter{
		Requests:     map[string]FakeElasticsearchRequests{},
		RequestOrder: map[string]int{},
		Responses:    fres,
		seqNo:        1,
	}
}

type FakeElasticsearchChatter struct {
	seqNo        int
	Requests     map[string]FakeElasticsearchRequests
	RequestOrder map[string]int
	Responses    map[string]FakeElasticsearchResponses
}

type FakeElasticsearchRequests []FakeElasticsearchRequest

type FakeElasticsearchRequest struct {
	URI    string
	Method string
	Body   string
	SeqNo  int
}

type FakeElasticsearchResponses []FakeElasticsearchResponse

type FakeElasticsearchResponse struct {
	Error      error
	StatusCode int
	Body       string
}

func (chat *FakeElasticsearchChatter) GetRequest(key string) (*FakeElasticsearchRequest, bool) {
	requests, found := chat.Requests[key]
	if !found {
		return nil, found
	}

	request := requests[0]
	chat.Requests[key] = requests[1:]
	return &request, found
}

func (chat *FakeElasticsearchChatter) GetResponse(key string) (*FakeElasticsearchResponse, bool) {
	responses, found := chat.Responses[key]
	if !found {
		return nil, found
	}

	response := responses[0]
	chat.Responses[key] = responses[1:]
	return &response, found
}

func (chat *FakeElasticsearchChatter) recordRequest(payload *esclient.EsRequest) {
	key := payload.URI
	req := FakeElasticsearchRequest{
		URI:    key,
		Body:   payload.RequestBody,
		Method: payload.Method,
		SeqNo:  chat.nextSeqNo(),
	}
	chat.Requests[key] = append(chat.Requests[key], req)
}

func (chat *FakeElasticsearchChatter) nextSeqNo() int {
	next := chat.seqNo
	chat.seqNo = chat.seqNo + 1
	return next
}

func (response *FakeElasticsearchResponse) BodyAsResponseBody() map[string]interface{} {
	body := map[string]interface{}{}
	if err := json.Unmarshal([]byte(response.Body), &body); err != nil {
		body = make(map[string]interface{})
		body["results"] = response.Body
	}
	return body
}

func NewFakeElasticsearchClient(cluster, namespace string, k8sClient client.Client, chatter *FakeElasticsearchChatter) esclient.Client {
	sendFakeRequest := NewFakeSendRequestFn(chatter)
	c := esclient.NewClient(log.DefaultLogger(), cluster, namespace, k8sClient)
	c.SetSendRequestFn(sendFakeRequest)
	return c
}

func NewFakeSendRequestFn(chatter *FakeElasticsearchChatter) esclient.FnEsSendRequest {
	return func(log logr.Logger, cluster, namespace string, payload *esclient.EsRequest, client client.Client) {
		chatter.recordRequest(payload)
		if val, found := chatter.GetResponse(payload.URI); found {
			payload.Error = val.Error
			payload.StatusCode = val.StatusCode
			payload.RawResponseBody = val.Body
			payload.ResponseBody = val.BodyAsResponseBody()
		} else {
			payload.Error = kverrors.New("No fake response found for uri",
				"uri", payload.URI,
				"payload", payload)
		}
	}
}
