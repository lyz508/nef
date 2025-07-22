package processor

import (
	"net/http"

	"github.com/free5gc/nef/internal/logger"
	"github.com/free5gc/nef/pkg/factory"
	"github.com/free5gc/openapi"
	"github.com/free5gc/openapi/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func (p *Processor) GetTrafficInfluenceSubscription(
	c *gin.Context,
	afID string,
) {
	logger.TrafInfluLog.Infof("GetTrafficInfluenceSubscription - afID[%s]", afID)

	af := p.Context().GetAf(afID)
	if af == nil {
		pd := openapi.ProblemDetailsDataNotFound("AF is not found")
		c.JSON(http.StatusNotFound, pd)
		return
	}

	af.Mu.RLock()
	defer af.Mu.RUnlock()

	var tiSubs []models.NefTrafficInfluSub
	for _, sub := range af.Subs {
		if sub.TiSub == nil {
			continue
		}
		tiSubs = append(tiSubs, *sub.TiSub)
	}
	c.JSON(http.StatusOK, &tiSubs)
}

func (p *Processor) PostTrafficInfluenceSubscription(
	c *gin.Context,
	afID string,
	tiSub *models.NefTrafficInfluSub,
) {
	logger.TrafInfluLog.Infof("PostTrafficInfluenceSubscription - afID[%s]", afID)

	rsp := validateTrafficInfluenceData(tiSub)
	if rsp != nil {
		c.JSON(rsp.Status, rsp.Body)
		return
	}

	nefCtx := p.Context()
	af := nefCtx.GetAf(afID)
	if af == nil {
		af = nefCtx.NewAf(afID)
		if af == nil {
			pd := openapi.ProblemDetailsSystemFailure("No resource can be allocated")
			c.JSON(int(pd.Status), pd)
			return
		}
	}

	af.Mu.Lock()
	defer af.Mu.Unlock()

	correID := nefCtx.NewCorreID()
	afSub := af.NewSub(correID, tiSub)
	if afSub == nil {
		pd := openapi.ProblemDetailsSystemFailure("No resource can be allocated")
		c.JSON(int(pd.Status), pd)
		return
	}

	if len(tiSub.Gpsi) > 0 || len(tiSub.Ipv4Addr) > 0 || len(tiSub.Ipv6Addr) > 0 {
		// Single UE, sent to PCF
		asc := p.convertTrafficInfluSubToAppSessionContext(tiSub, afSub.NotifCorreID)
		rspStatus, rspBody, appSessID := p.Consumer().PostAppSessions(asc)
		if rspStatus != http.StatusCreated {
			c.JSON(rspStatus, rspBody)
			return
		}
		afSub.AppSessID = appSessID
	} else if len(tiSub.ExternalGroupId) > 0 || tiSub.AnyUeInd {
		// Group or any UE, sent to UDR
		afSub.InfluID = uuid.New().String()
		tiData := p.convertTrafficInfluSubToTrafficInfluData(tiSub, afSub.NotifCorreID)
		rspStatus, rspBody := p.Consumer().AppDataInfluenceDataPut(afSub.InfluID, tiData)
		if rspStatus != http.StatusOK &&
			rspStatus != http.StatusCreated &&
			rspStatus != http.StatusNoContent {
			c.JSON(rspStatus, rspBody)
			return
		}
	} else {
		// Invalid case. Return Error
		pd := openapi.ProblemDetailsMalformedReqSyntax("Not individual UE case, nor group case")
		c.JSON(int(pd.Status), pd)
		return
	}

	af.Subs[afSub.SubID] = afSub
	af.Log.Infoln("Subscription is added")

	nefCtx.AddAf(af)

	// Create Location URI
	tiSub.Self = p.genTrafficInfluSubURI(afID, afSub.SubID)
	headers := map[string][]string{
		"Location": {tiSub.Self},
	}

	for hdrName, hdrValues := range headers {
		for _, hdrValue := range hdrValues {
			c.Header(hdrName, hdrValue)
		}
	}
	af.Log.Infoln("Convert TI 3")
	c.JSON(http.StatusCreated, tiSub)
}

func (p *Processor) GetIndividualTrafficInfluenceSubscription(
	c *gin.Context,
	afID, subID string,
) {
	logger.TrafInfluLog.Infof("GetIndividualTrafficInfluenceSubscription - afID[%s], subID[%s]", afID, subID)

	af := p.Context().GetAf(afID)
	if af == nil {
		pd := openapi.ProblemDetailsDataNotFound("AF is not found")
		c.JSON(http.StatusNotFound, pd)
		return
	}

	af.Mu.RLock()
	defer af.Mu.RUnlock()

	afSub, ok := af.Subs[subID]
	if !ok {
		pd := openapi.ProblemDetailsDataNotFound("Subscription is not found")
		c.JSON(http.StatusNotFound, pd)
		return
	}

	c.JSON(http.StatusOK, afSub.TiSub)
}

func (p *Processor) PutIndividualTrafficInfluenceSubscription(
	c *gin.Context,
	afID, subID string,
	tiSub *models.NefTrafficInfluSub,
) {
	logger.TrafInfluLog.Infof("PutIndividualTrafficInfluenceSubscription - afID[%s], subID[%s]", afID, subID)

	rsp := validateTrafficInfluenceData(tiSub)
	if rsp != nil {
		c.JSON(rsp.Status, rsp.Body)
		return
	}

	af := p.Context().GetAf(afID)
	if af == nil {
		pd := openapi.ProblemDetailsDataNotFound("AF is not found")
		c.JSON(http.StatusNotFound, pd)
		return
	}

	af.Mu.Lock()
	defer af.Mu.Unlock()

	afSub, ok := af.Subs[subID]
	if !ok {
		pd := openapi.ProblemDetailsDataNotFound("Subscription is not found")
		c.JSON(http.StatusNotFound, pd)
		return
	}

	afSub.TiSub = tiSub
	if afSub.AppSessID != "" {
		asc := p.convertTrafficInfluSubToAppSessionContext(tiSub, afSub.NotifCorreID)
		rspStatus, rspBody, appSessID := p.Consumer().PostAppSessions(asc)
		if rspStatus != http.StatusCreated {
			c.JSON(rspStatus, rspBody)
			return
		}
		afSub.AppSessID = appSessID
	} else if afSub.InfluID != "" {
		tiData := p.convertTrafficInfluSubToTrafficInfluData(tiSub, afSub.NotifCorreID)
		rspStatus, rspBody := p.Consumer().AppDataInfluenceDataPut(afSub.InfluID, tiData)
		if rspStatus != http.StatusOK &&
			rspStatus != http.StatusCreated &&
			rspStatus != http.StatusNoContent {
			c.JSON(rspStatus, rspBody)
			return
		}
	} else {
		pd := openapi.ProblemDetailsDataNotFound("No AppSessID or InfluID")
		c.JSON(int(pd.Status), pd)
		return
	}

	c.JSON(http.StatusOK, afSub.TiSub)
}

func (p *Processor) PatchIndividualTrafficInfluenceSubscription(
	c *gin.Context,
	afID, subID string,
	tiSubPatch *models.NefTrafficInfluSubPatch,
) {
	logger.TrafInfluLog.Infof("PatchIndividualTrafficInfluenceSubscription - afID[%s], subID[%s]", afID, subID)

	af := p.Context().GetAf(afID)
	if af == nil {
		pd := openapi.ProblemDetailsDataNotFound("AF is not found")
		c.JSON(http.StatusNotFound, pd)
		return
	}

	af.Mu.Lock()
	defer af.Mu.Unlock()

	afSub, ok := af.Subs[subID]
	if !ok {
		pd := openapi.ProblemDetailsDataNotFound("Subscription is not found")
		c.JSON(http.StatusNotFound, pd)
		return
	}

	if afSub.AppSessID != "" {
		ascUpdateData := p.convertTrafficInfluSubPatchToAppSessionContextUpdateData(tiSubPatch)
		rspStatus, rspBody := p.Consumer().PatchAppSession(afSub.AppSessID, ascUpdateData)
		if rspStatus != http.StatusOK &&
			rspStatus != http.StatusNoContent {
			c.JSON(rspStatus, rspBody)
			return
		}
	} else if afSub.InfluID != "" {
		tiDataPatch := p.convertTrafficInfluSubPatchToTrafficInfluDataPatch(tiSubPatch)
		rspStatus, rspBody := p.Consumer().AppDataInfluenceDataPatch(afSub.InfluID, tiDataPatch)
		if rspStatus != http.StatusOK &&
			rspStatus != http.StatusNoContent {
			c.JSON(rspStatus, rspBody)
			return
		}
	} else {
		pd := openapi.ProblemDetailsDataNotFound("No AppSessID or InfluID")
		c.JSON(int(pd.Status), pd)
		return
	}

	afSub.PatchTiSubData(tiSubPatch)
	c.JSON(http.StatusOK, afSub.TiSub)
}

func (p *Processor) DeleteIndividualTrafficInfluenceSubscription(
	c *gin.Context,
	afID, subID string,
) {
	logger.TrafInfluLog.Infof("DeleteIndividualTrafficInfluenceSubscription - afID[%s], subID[%s]", afID, subID)

	af := p.Context().GetAf(afID)
	if af == nil {
		pd := openapi.ProblemDetailsDataNotFound("AF is not found")
		c.JSON(http.StatusNotFound, pd)
		return
	}

	af.Mu.Lock()
	defer af.Mu.Unlock()

	sub, ok := af.Subs[subID]
	if !ok {
		pd := openapi.ProblemDetailsDataNotFound("Subscription is not found")
		c.JSON(http.StatusNotFound, pd)
		return
	}

	if sub.AppSessID != "" {
		rspStatus, rspBody := p.Consumer().DeleteAppSession(sub.AppSessID)
		if rspStatus != http.StatusOK &&
			rspStatus != http.StatusNoContent {
			c.JSON(rspStatus, rspBody)
			return
		}
	} else {
		rspStatus, rspBody := p.Consumer().AppDataInfluenceDataDelete(sub.InfluID)
		if rspStatus != http.StatusOK &&
			rspStatus != http.StatusNoContent {
			c.JSON(rspStatus, rspBody)
			return
		}
	}
	delete(af.Subs, subID)
	c.JSON(http.StatusNoContent, nil)
}

func validateTrafficInfluenceData(
	tiSub *models.NefTrafficInfluSub,
) *HandlerResponse {
	// TS29.522: One of "afAppId", "trafficFilters" or "ethTrafficFilters" shall be included.
	if tiSub.AfAppId == "" &&
		len(tiSub.TrafficFilters) == 0 &&
		len(tiSub.EthTrafficFilters) == 0 {
		pd := openapi.
			ProblemDetailsMalformedReqSyntax(
				"Missing one of afAppId, trafficFilters or ethTrafficFilters")
		return &HandlerResponse{int(pd.Status), nil, pd}
	}

	// TS29.522: One of individual UE identifier
	// (i.e. "gpsi", “macAddr”, "ipv4Addr" or "ipv6Addr"),
	// External Group Identifier (i.e. "externalGroupId") or
	// any UE indication "anyUeInd" shall be included.
	if tiSub.Gpsi == "" &&
		tiSub.Ipv4Addr == "" &&
		tiSub.Ipv6Addr == "" &&
		tiSub.ExternalGroupId == "" &&
		!tiSub.AnyUeInd {
		pd := openapi.
			ProblemDetailsMalformedReqSyntax(
				"Missing one of Gpsi, Ipv4Addr, Ipv6Addr, ExternalGroupId, AnyUeInd")
		return &HandlerResponse{int(pd.Status), nil, pd}
	}
	return nil
}

func (p *Processor) genTrafficInfluSubURI(
	afID, subscriptionId string,
) string {
	// E.g. https://localhost:29505/3gpp-traffic-Influence/v1/{afId}/subscriptions/{subscriptionId}
	return p.Config().ServiceUri(factory.ServiceTraffInflu) + "/" + afID + "/subscriptions/" + subscriptionId
}

func (p *Processor) genNotificationUri() string {
	return p.Config().ServiceUri(factory.ServiceNefCallback) + "/notification/smf"
}

func (p *Processor) convertTrafficInfluSubToAppSessionContext(
	tiSub *models.NefTrafficInfluSub,
	notifCorreID string,
) *models.AppSessionContext {
	asc := &models.AppSessionContext{
		AscReqData: &models.AppSessionContextReqData{
			AfAppId: tiSub.AfAppId,
			AfRoutReq: &models.AfRoutingRequirement{
				AppReloc:    tiSub.AppReloInd,
				RouteToLocs: tiSub.TrafficRoutes,
				TempVals:    tiSub.TempValidities,
			},
			UeIpv4:    tiSub.Ipv4Addr,
			UeIpv6:    tiSub.Ipv6Addr,
			UeMac:     tiSub.MacAddr,
			NotifUri:  tiSub.NotificationDestination,
			SuppFeat:  tiSub.SuppFeat,
			Dnn:       tiSub.Dnn,
			SliceInfo: tiSub.Snssai,
			// Supi: ,
		},
	}

	if tiSub.DnaiChgType != "" {
		asc.AscReqData.AfRoutReq.UpPathChgSub = &models.UpPathChgEvent{
			DnaiChgType:     tiSub.DnaiChgType,
			NotificationUri: p.genNotificationUri(),
			NotifCorreId:    notifCorreID,
		}
	}
	return asc
}

func (p *Processor) convertTrafficInfluSubPatchToAppSessionContextUpdateData(
	tiSubPatch *models.NefTrafficInfluSubPatch,
) *models.AppSessionContextUpdateData {
	ascUpdate := &models.AppSessionContextUpdateData{
		AfRoutReq: &models.AfRoutingRequirementRm{
			AppReloc:    tiSubPatch.AppReloInd,
			RouteToLocs: tiSubPatch.TrafficRoutes,
			TempVals:    tiSubPatch.TempValidities,
		},
	}
	return ascUpdate
}

func (p *Processor) convertTrafficInfluSubToTrafficInfluData(
	tiSub *models.NefTrafficInfluSub,
	notifCorreID string,
) *models.TrafficInfluData {
	tiData := &models.TrafficInfluData{
		AfAppId:    tiSub.AfAppId,
		AppReloInd: tiSub.AppReloInd,
		// Supi: ,
		DnaiChgType:           tiSub.DnaiChgType,
		UpPathChgNotifUri:     p.genNotificationUri(),
		UpPathChgNotifCorreId: notifCorreID,
		Dnn:                   tiSub.Dnn,
		Snssai:                tiSub.Snssai,
		EthTrafficFilters:     tiSub.EthTrafficFilters,
		TrafficFilters:        tiSub.TrafficFilters,
		TrafficRoutes:         tiSub.TrafficRoutes,
		TraffCorreInd:         tiSub.TfcCorrInd,
		// ValidStartTime: ,
		// ValidEndTime: ,
		TempValidities:    tiSub.TempValidities,
		AfAckInd:          tiSub.AfAckInd,
		AddrPreserInd:     tiSub.AddrPreserInd,
		SupportedFeatures: tiSub.SuppFeat,
	}

	// TODO: handle ExternalGroupId
	if tiSub.AnyUeInd {
		tiData.InterGroupId = "AnyUE"
	}

	return tiData
}

func (p *Processor) convertTrafficInfluSubPatchToTrafficInfluDataPatch(
	tiSubPatch *models.NefTrafficInfluSubPatch,
) *models.TrafficInfluDataPatch {
	tiDataPatch := &models.TrafficInfluDataPatch{
		AppReloInd:        tiSubPatch.AppReloInd,
		EthTrafficFilters: tiSubPatch.EthTrafficFilters,
		TrafficFilters:    tiSubPatch.TrafficFilters,
		TrafficRoutes:     tiSubPatch.TrafficRoutes,
	}
	return tiDataPatch
}
