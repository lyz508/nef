package consumer

import (
	ctx "context"
	"net/http"
	"strings"
	"sync"

	"github.com/antihax/optional"

	"bitbucket.org/free5gc-team/nef/internal/context"
	"bitbucket.org/free5gc-team/nef/internal/factory"
	"bitbucket.org/free5gc-team/nef/internal/logger"
	"bitbucket.org/free5gc-team/openapi/Nnrf_NFDiscovery"
	"bitbucket.org/free5gc-team/openapi/Npcf_PolicyAuthorization"
	"bitbucket.org/free5gc-team/openapi/models"
)

type ConsumerPCFService struct {
	cfg              *factory.Config
	nefCtx           *context.NefContext
	nrfSrv           *ConsumerNRFService
	clientPolicyAuth *Npcf_PolicyAuthorization.APIClient
	clientMtx        sync.RWMutex
}

const ServiceName_NPCF_POLICYAUTHORIZATION string = "npcf-policyauthorization"

func NewConsumerPCFService(nefCfg *factory.Config, nefCtx *context.NefContext,
	nrfSrv *ConsumerNRFService) *ConsumerPCFService {

	c := &ConsumerPCFService{cfg: nefCfg, nefCtx: nefCtx, nrfSrv: nrfSrv}
	return c
}

func (c *ConsumerPCFService) initPolicyAuthAPIClient() error {
	c.clientMtx.Lock()
	defer c.clientMtx.Unlock()

	if c.clientPolicyAuth != nil {
		return nil
	}

	param := Nnrf_NFDiscovery.SearchNFInstancesParamOpts{
		ServiceNames: optional.NewInterface([]string{ServiceName_NPCF_POLICYAUTHORIZATION}),
	}
	uri, err := c.nrfSrv.SearchNFServiceUri("PCF", ServiceName_NPCF_POLICYAUTHORIZATION, &param)
	if err != nil {
		logger.ConsumerLog.Errorf(err.Error())
		return err
	}
	logger.ConsumerLog.Infof("initPolicyAuthAPIClient: uri[%s]", uri)

	//TODO: Subscribe NRF to notify service URI change

	paCfg := Npcf_PolicyAuthorization.NewConfiguration()
	paCfg.SetBasePath(uri)
	c.clientPolicyAuth = Npcf_PolicyAuthorization.NewAPIClient(paCfg)
	return nil
}

func (c *ConsumerPCFService) GetAppSession(appSessionId string) (int, interface{}) {
	var (
		err     error
		rspCode int
		rspBody interface{}
		result  models.AppSessionContext
		rsp     *http.Response
	)

	if err = c.initPolicyAuthAPIClient(); err != nil {
		return rspCode, rspBody
	}

	c.clientMtx.RLock()
	result, rsp, err = c.clientPolicyAuth.IndividualApplicationSessionContextDocumentApi.
		GetAppSession(ctx.Background(), appSessionId)
	c.clientMtx.RUnlock()

	if rsp != nil {
		rspCode = rsp.StatusCode
		if rsp.StatusCode == http.StatusOK {
			rspBody = &result
		} else if err != nil {
			rspCode, rspBody = handleAPIServiceResponseError(rsp, err)
		}
	} else {
		//API Service Internal Error or Server No Response
		rspCode, rspBody = handleAPIServiceNoResponse(err)
	}

	return rspCode, rspBody
}

func (c *ConsumerPCFService) PostAppSessions(asc *models.AppSessionContext) (int, interface{}, string) {
	var (
		err       error
		rspCode   int
		rspBody   interface{}
		appSessID string
		result    models.AppSessionContext
		rsp       *http.Response
	)

	if err = c.initPolicyAuthAPIClient(); err != nil {
		return rspCode, rspBody, appSessID
	}

	c.clientMtx.RLock()
	result, rsp, err = c.clientPolicyAuth.ApplicationSessionsCollectionApi.PostAppSessions(ctx.Background(), *asc)
	c.clientMtx.RUnlock()

	if rsp != nil {
		rspCode = rsp.StatusCode
		if rsp.StatusCode == http.StatusCreated {
			logger.ConsumerLog.Debugf("PostAppSessions RspData: %+v", result)
			rspBody = &result
			appSessID = getAppSessIDFromRspLocationHeader(rsp)
		} else if err != nil {
			rspCode, rspBody = handleAPIServiceResponseError(rsp, err)
		}
	} else {
		//API Service Internal Error or Server No Response
		rspCode, rspBody = handleAPIServiceNoResponse(err)
	}

	return rspCode, rspBody, appSessID
}

func (c *ConsumerPCFService) PutAppSession(appSessionId string, ascUpdateData *models.AppSessionContextUpdateData, asc *models.AppSessionContext) (int, interface{}, string) {
	var (
		err       error
		rspCode   int
		rspBody   interface{}
		appSessID string
		result    models.AppSessionContext
		rsp       *http.Response
	)

	if err = c.initPolicyAuthAPIClient(); err != nil {
		return rspCode, rspBody, appSessID
	}

	appSessID = appSessionId
	c.clientMtx.RLock()
	result, rsp, err = c.clientPolicyAuth.IndividualApplicationSessionContextDocumentApi.
		GetAppSession(ctx.Background(), appSessionId)
	c.clientMtx.RUnlock()

	if rsp != nil {
		rspCode = rsp.StatusCode
		if rsp.StatusCode == http.StatusOK {
			// Patch
			c.clientMtx.RLock()
			result, rsp, err = c.clientPolicyAuth.IndividualApplicationSessionContextDocumentApi.ModAppSession(ctx.Background(), appSessionId, *ascUpdateData)
			c.clientMtx.RUnlock()

			if rsp != nil {
				rspCode = rsp.StatusCode
				if rsp.StatusCode == http.StatusOK {
					logger.ConsumerLog.Debugf("PatchAppSessions RspData: %+v", result)
					rspBody = &result
				} else if err != nil {
					rspCode, rspBody = handleAPIServiceResponseError(rsp, err)
				}
			} else {
				//API Service Internal Error or Server No Response
				rspCode, rspBody = handleAPIServiceNoResponse(err)
			}

			return rspCode, rspBody, appSessID
		} else if err != nil {
			//Post
		}
	} else {
		//API Service Internal Error or Server No Response
		rspCode, rspBody = handleAPIServiceNoResponse(err)
		return rspCode, rspBody, appSessID
	}

	return rspCode, rspBody, appSessID
}

func (c *ConsumerPCFService) PatchAppSession(appSessionId string, ascUpdateData *models.AppSessionContextUpdateData) (int, interface{}) {
	var (
		err     error
		rspCode int
		rspBody interface{}
		result  models.AppSessionContext
		rsp     *http.Response
	)

	if err = c.initPolicyAuthAPIClient(); err != nil {
		return rspCode, rspBody
	}

	c.clientMtx.RLock()
	result, rsp, err = c.clientPolicyAuth.IndividualApplicationSessionContextDocumentApi.ModAppSession(ctx.Background(), appSessionId, *ascUpdateData)
	c.clientMtx.RUnlock()

	if rsp != nil {
		rspCode = rsp.StatusCode
		if rsp.StatusCode == http.StatusOK {
			logger.ConsumerLog.Debugf("PatchAppSessions RspData: %+v", result)
			rspBody = &result
		} else if err != nil {
			rspCode, rspBody = handleAPIServiceResponseError(rsp, err)
		}
	} else {
		//API Service Internal Error or Server No Response
		rspCode, rspBody = handleAPIServiceNoResponse(err)
	}

	return rspCode, rspBody
}

func (c *ConsumerPCFService) DeleteAppSession(appSessionId string) (int, interface{}) {
	var (
		err     error
		rspCode int
		rspBody interface{}
		result  models.AppSessionContext
		rsp     *http.Response
	)

	if err = c.initPolicyAuthAPIClient(); err != nil {
		return rspCode, rspBody
	}

	param := &Npcf_PolicyAuthorization.DeleteAppSessionParamOpts{
		EventsSubscReqData: optional.NewInterface(models.EventsSubscReqData{}),
	}

	c.clientMtx.RLock()
	result, rsp, err = c.clientPolicyAuth.IndividualApplicationSessionContextDocumentApi.DeleteAppSession(ctx.Background(), appSessionId, param)
	c.clientMtx.RUnlock()

	if rsp != nil {
		rspCode = rsp.StatusCode
		if rsp.StatusCode == http.StatusOK {
			logger.ConsumerLog.Debugf("DeleteAppSessions RspData: %+v", result)
			rspBody = &result
		} else if err != nil {
			rspCode, rspBody = handleAPIServiceResponseError(rsp, err)
		}
	} else {
		//API Service Internal Error or Server No Response
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
