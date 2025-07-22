package consumer

import (
	"net/http"
	"strings"
	"sync"

	"github.com/free5gc/nef/internal/logger"
	"github.com/free5gc/openapi/models"
	"github.com/free5gc/openapi/nrf/NFDiscovery"
	"github.com/free5gc/openapi/pcf/PolicyAuthorization"
)

type npcfService struct {
	consumer *Consumer

	mu      sync.RWMutex
	clients map[string]*PolicyAuthorization.APIClient
}

func (s *npcfService) getClient(uri string) *PolicyAuthorization.APIClient {
	s.mu.RLock()
	if client, ok := s.clients[uri]; ok {
		defer s.mu.RUnlock()
		return client
	} else {
		configuration := PolicyAuthorization.NewConfiguration()
		configuration.SetBasePath(uri)
		cli := PolicyAuthorization.NewAPIClient(configuration)

		s.mu.RUnlock()
		s.mu.Lock()
		defer s.mu.Unlock()
		s.clients[uri] = cli
		return cli
	}
}

func (s *npcfService) getPcfPolicyAuthUri() (string, error) {
	uri := s.consumer.Context().PcfPaUri()
	if uri == "" {
		localVarOptionals := NFDiscovery.SearchNFInstancesRequest{}
		logger.ConsumerLog.Infoln(s.consumer.Config().NrfUri())
		_, sUri, err := s.consumer.SearchNFInstances(s.consumer.Config().NrfUri(), models.ServiceName_NPCF_POLICYAUTHORIZATION,
			models.NrfNfManagementNfType_PCF, models.NrfNfManagementNfType_NEF, &localVarOptionals)
		if err == nil {
			s.consumer.Context().SetPcfPaUri(sUri)
		}
		return sUri, err
	}
	return uri, nil
}

func (s *npcfService) GetAppSession(appSessionId string) (int, interface{}) {
	var (
		err     error
		rspCode int
		rspBody interface{}
		rsp     *PolicyAuthorization.GetAppSessionResponse
	)

	uri, err := s.getPcfPolicyAuthUri()
	if err != nil {
		return rspCode, rspBody
	}
	client := s.getClient(uri)

	ctx, _, err := s.consumer.Context().GetTokenCtx(models.ServiceName_NPCF_POLICYAUTHORIZATION, models.NrfNfManagementNfType_PCF)
	if err != nil {
		return rspCode, rspBody
	}

	appSessReq := &PolicyAuthorization.GetAppSessionRequest{
		AppSessionId: &appSessionId,
	}
	rsp, err = client.IndividualApplicationSessionContextDocumentApi.
		GetAppSession(ctx, appSessReq)

	if rsp != nil {
		rspCode = http.StatusOK
		rspBody = rsp.AppSessionContext
	} else {
		rspCode, rspBody = handleAPIServiceNoResponse(err)
	}

	return rspCode, rspBody
}

func (s *npcfService) PostAppSessions(asc *models.AppSessionContext) (int, interface{}, string) {
	var (
		err       error
		rspCode   int
		rspBody   interface{}
		appSessID string
		rsp       *PolicyAuthorization.PostAppSessionsResponse
	)

	uri, err := s.getPcfPolicyAuthUri()
	if err != nil {
		return rspCode, rspBody, appSessID
	}
	client := s.getClient(uri)

	ctx, _, err := s.consumer.Context().GetTokenCtx(models.ServiceName_NPCF_POLICYAUTHORIZATION, models.NrfNfManagementNfType_PCF)
	if err != nil {
		return rspCode, rspBody, appSessID
	}

	req := &PolicyAuthorization.PostAppSessionsRequest{
		AppSessionContext: asc,
	}
	rsp, err = client.ApplicationSessionsCollectionApi.PostAppSessions(ctx, req)

	if rsp != nil {
		rspCode = http.StatusCreated
		rspBody = rsp.AppSessionContext
		appSessID = rsp.Location
		logger.ConsumerLog.Debugf("PostAppSessions RspData: %+v", rsp.AppSessionContext)
	} else {
		rspCode, rspBody = handleAPIServiceNoResponse(err)
	}

	return rspCode, rspBody, appSessID
}

func (s *npcfService) PutAppSession(
	appSessionId string,
	ascUpdateData *models.AppSessionContextUpdateData,
	asc *models.AppSessionContext,
) (int, interface{}, string) {
	var (
		err       error
		rspCode   int
		rspBody   interface{}
		appSessID string
		rsp       *PolicyAuthorization.GetAppSessionResponse
		modRsp    *PolicyAuthorization.ModAppSessionResponse
	)

	uri, err := s.getPcfPolicyAuthUri()
	if err != nil {
		return rspCode, rspBody, appSessID
	}
	client := s.getClient(uri)

	ctx, _, err := s.consumer.Context().GetTokenCtx(models.ServiceName_NPCF_POLICYAUTHORIZATION, models.NrfNfManagementNfType_PCF)
	if err != nil {
		return rspCode, rspBody, appSessID
	}

	appSessReq := &PolicyAuthorization.GetAppSessionRequest{
		AppSessionId: &appSessionId,
	}
	rsp, err = client.IndividualApplicationSessionContextDocumentApi.
		GetAppSession(ctx, appSessReq)

	if rsp != nil {
		rspCode = http.StatusOK

		appSessModReq := &PolicyAuthorization.ModAppSessionRequest{
			AppSessionId: &appSessID,
			AppSessionContextUpdateDataPatch: &models.AppSessionContextUpdateDataPatch{
				AscReqData: ascUpdateData,
			},
		}
		modRsp, err = client.IndividualApplicationSessionContextDocumentApi.ModAppSession(ctx, appSessModReq)

		if modRsp != nil {
			rspCode = http.StatusOK
		} else {
			rspCode, rspBody = handleAPIServiceNoResponse(err)
		}
	} else {
		// API Service Internal Error or Server No Response
		rspCode, rspBody = handleAPIServiceNoResponse(err)
	}

	return rspCode, rspBody, appSessID
}

func (s *npcfService) PatchAppSession(appSessionId string,
	ascUpdateData *models.AppSessionContextUpdateData,
) (int, interface{}) {
	var (
		err     error
		rspCode int
		rspBody interface{}
		rsp     *PolicyAuthorization.ModAppSessionResponse
	)

	uri, err := s.getPcfPolicyAuthUri()
	if err != nil {
		return rspCode, rspBody
	}
	client := s.getClient(uri)

	ctx, _, err := s.consumer.Context().GetTokenCtx(models.ServiceName_NPCF_POLICYAUTHORIZATION, models.NrfNfManagementNfType_PCF)
	if err != nil {
		return rspCode, rspBody
	}

	appSessModReq := &PolicyAuthorization.ModAppSessionRequest{
		AppSessionId: &appSessionId,
		AppSessionContextUpdateDataPatch: &models.AppSessionContextUpdateDataPatch{
			AscReqData: ascUpdateData,
		},
	}
	rsp, err = client.IndividualApplicationSessionContextDocumentApi.ModAppSession(
		ctx, appSessModReq)

	if rsp != nil {
		rspCode = http.StatusOK
		rspBody = rsp.AppSessionContext
		logger.ConsumerLog.Debugf("PatchAppSessions RspData: %+v", rsp)
	} else {
		rspCode, rspBody = handleAPIServiceNoResponse(err)
	}

	return rspCode, rspBody
}

func (s *npcfService) DeleteAppSession(appSessionId string) (int, interface{}) {
	var (
		err     error
		rspCode int
		rspBody interface{}
		rsp     *PolicyAuthorization.DeleteAppSessionResponse
	)

	uri, err := s.getPcfPolicyAuthUri()
	if err != nil {
		return rspCode, rspBody
	}
	client := s.getClient(uri)

	// param := &PolicyAuthorization.DeleteAppSessionParamOpts{
	// 	EventsSubscReqData: optional.NewInterface(models.EventsSubscReqData{}),
	// }

	ctx, _, err := s.consumer.Context().GetTokenCtx(models.ServiceName_NPCF_POLICYAUTHORIZATION, models.NrfNfManagementNfType_PCF)
	if err != nil {
		return rspCode, rspBody
	}

	appSessDelReq := &PolicyAuthorization.DeleteAppSessionRequest{
		AppSessionId: &appSessionId,
	}
	rsp, err = client.IndividualApplicationSessionContextDocumentApi.DeleteAppSession(
		ctx, appSessDelReq)

	if rsp != nil {
		rspCode = http.StatusNoContent
		logger.ConsumerLog.Debugf("DeleteAppSessions RspData: %+v", rsp)
	} else {
		rspCode, rspBody = handleAPIServiceNoResponse(err)
	}

	return rspCode, rspBody
}

func getAppSessIDFromRspLocationHeader(rsp *http.Response) string {
	appSessID := ""
	loc := rsp.Header.Get("Location")
	if strings.Contains(loc, "http") {
		index := strings.LastIndex(loc, "/")
		appSessID = loc[index+1:]
	}
	logger.ConsumerLog.Infof("appSessID=%q", appSessID)
	return appSessID
}
