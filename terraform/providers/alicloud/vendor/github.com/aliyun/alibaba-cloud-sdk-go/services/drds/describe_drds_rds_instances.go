package drds

//Licensed under the Apache License, Version 2.0 (the "License");
//you may not use this file except in compliance with the License.
//You may obtain a copy of the License at
//
//http://www.apache.org/licenses/LICENSE-2.0
//
//Unless required by applicable law or agreed to in writing, software
//distributed under the License is distributed on an "AS IS" BASIS,
//WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//See the License for the specific language governing permissions and
//limitations under the License.
//
// Code generated by Alibaba Cloud SDK Code Generator.
// Changes may cause incorrect behavior and will be lost if the code is regenerated.

import (
	"github.com/aliyun/alibaba-cloud-sdk-go/sdk/requests"
	"github.com/aliyun/alibaba-cloud-sdk-go/sdk/responses"
)

// DescribeDrdsRdsInstances invokes the drds.DescribeDrdsRdsInstances API synchronously
func (client *Client) DescribeDrdsRdsInstances(request *DescribeDrdsRdsInstancesRequest) (response *DescribeDrdsRdsInstancesResponse, err error) {
	response = CreateDescribeDrdsRdsInstancesResponse()
	err = client.DoAction(request, response)
	return
}

// DescribeDrdsRdsInstancesWithChan invokes the drds.DescribeDrdsRdsInstances API asynchronously
func (client *Client) DescribeDrdsRdsInstancesWithChan(request *DescribeDrdsRdsInstancesRequest) (<-chan *DescribeDrdsRdsInstancesResponse, <-chan error) {
	responseChan := make(chan *DescribeDrdsRdsInstancesResponse, 1)
	errChan := make(chan error, 1)
	err := client.AddAsyncTask(func() {
		defer close(responseChan)
		defer close(errChan)
		response, err := client.DescribeDrdsRdsInstances(request)
		if err != nil {
			errChan <- err
		} else {
			responseChan <- response
		}
	})
	if err != nil {
		errChan <- err
		close(responseChan)
		close(errChan)
	}
	return responseChan, errChan
}

// DescribeDrdsRdsInstancesWithCallback invokes the drds.DescribeDrdsRdsInstances API asynchronously
func (client *Client) DescribeDrdsRdsInstancesWithCallback(request *DescribeDrdsRdsInstancesRequest, callback func(response *DescribeDrdsRdsInstancesResponse, err error)) <-chan int {
	result := make(chan int, 1)
	err := client.AddAsyncTask(func() {
		var response *DescribeDrdsRdsInstancesResponse
		var err error
		defer close(result)
		response, err = client.DescribeDrdsRdsInstances(request)
		callback(response, err)
		result <- 1
	})
	if err != nil {
		defer close(result)
		callback(nil, err)
		result <- 0
	}
	return result
}

// DescribeDrdsRdsInstancesRequest is the request struct for api DescribeDrdsRdsInstances
type DescribeDrdsRdsInstancesRequest struct {
	*requests.RpcRequest
	DrdsInstanceId string           `position:"Query" name:"DrdsInstanceId"`
	PageNumber     requests.Integer `position:"Query" name:"PageNumber"`
	PageSize       requests.Integer `position:"Query" name:"PageSize"`
	DbInstType     string           `position:"Query" name:"DbInstType"`
}

// DescribeDrdsRdsInstancesResponse is the response struct for api DescribeDrdsRdsInstances
type DescribeDrdsRdsInstancesResponse struct {
	*responses.BaseResponse
	RequestId   string                                `json:"RequestId" xml:"RequestId"`
	Success     bool                                  `json:"Success" xml:"Success"`
	PageNumber  string                                `json:"PageNumber" xml:"PageNumber"`
	PageSize    string                                `json:"PageSize" xml:"PageSize"`
	Total       string                                `json:"Total" xml:"Total"`
	DbInstances DbInstancesInDescribeDrdsRdsInstances `json:"DbInstances" xml:"DbInstances"`
}

// CreateDescribeDrdsRdsInstancesRequest creates a request to invoke DescribeDrdsRdsInstances API
func CreateDescribeDrdsRdsInstancesRequest() (request *DescribeDrdsRdsInstancesRequest) {
	request = &DescribeDrdsRdsInstancesRequest{
		RpcRequest: &requests.RpcRequest{},
	}
	request.InitWithApiInfo("Drds", "2019-01-23", "DescribeDrdsRdsInstances", "drds", "openAPI")
	request.Method = requests.POST
	return
}

// CreateDescribeDrdsRdsInstancesResponse creates a response to parse from DescribeDrdsRdsInstances response
func CreateDescribeDrdsRdsInstancesResponse() (response *DescribeDrdsRdsInstancesResponse) {
	response = &DescribeDrdsRdsInstancesResponse{
		BaseResponse: &responses.BaseResponse{},
	}
	return
}