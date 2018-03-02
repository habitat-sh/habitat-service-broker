package rest

import (
	"net/http"
	"strings"

	"github.com/golang/glog"
	"github.com/gorilla/mux"

	osb "github.com/pmorie/go-open-service-broker-client/v2"

	"github.com/pmorie/osb-broker-lib/pkg/broker"
	"github.com/pmorie/osb-broker-lib/pkg/metrics"
)

// APISurface is a type that describes a OSB REST API surface. APISurface is
// responsible for decoding HTTP requests and transforming them into the request
// object for each operation and transforming responses and errors returned from
// the broker's internal business logic into the correct places in the HTTP
// response.
type APISurface struct {
	// BusinessLogic contains the business logic that provides the
	// implementation for the different OSB API operations.
	BusinessLogic broker.BusinessLogic
	Metrics       *metrics.OSBMetricsCollector
}

// NewAPISurface returns a new, ready-to-go APISurface.
func NewAPISurface(businessLogic broker.BusinessLogic, m *metrics.OSBMetricsCollector) (*APISurface, error) {
	api := &APISurface{
		BusinessLogic: businessLogic,
		Metrics:       m,
	}

	return api, nil
}

// GetCatalogHandler is the mux handler that dispatches requests to get the
// broker's catalog to the broker's BusinessLogic.
func (s *APISurface) GetCatalogHandler(w http.ResponseWriter, r *http.Request) {
	s.Metrics.Actions.WithLabelValues("get_catalog").Inc()

	version := getBrokerAPIVersionFromRequest(r)
	if err := s.BusinessLogic.ValidateBrokerAPIVersion(version); err != nil {
		writeError(w, err, http.StatusPreconditionFailed)
		return
	}

	c := &broker.RequestContext{
		Writer:  w,
		Request: r,
	}

	response, err := s.BusinessLogic.GetCatalog(c)
	if err != nil {
		writeError(w, err, http.StatusInternalServerError)
		return
	}

	writeResponse(w, http.StatusOK, response)
}

// ProvisionHandler is the mux handler that dispatches ProvisionRequests to the
// broker's BusinessLogic.
func (s *APISurface) ProvisionHandler(w http.ResponseWriter, r *http.Request) {
	s.Metrics.Actions.WithLabelValues("provision").Inc()

	version := getBrokerAPIVersionFromRequest(r)
	if err := s.BusinessLogic.ValidateBrokerAPIVersion(version); err != nil {
		writeError(w, err, http.StatusPreconditionFailed)
		return
	}

	request, err := unpackProvisionRequest(r)
	if err != nil {
		writeError(w, err, http.StatusBadRequest)
		return
	}

	glog.Infof("Received ProvisionRequest for instanceID %q", request.InstanceID)

	c := &broker.RequestContext{
		Writer:  w,
		Request: r,
	}

	response, err := s.BusinessLogic.Provision(request, c)
	if err != nil {
		writeError(w, err, http.StatusInternalServerError)
		return
	}

	status := http.StatusOK
	if response.Async {
		status = http.StatusAccepted
	}

	writeResponse(w, status, response)
}

// unpackProvisionRequest unpacks an osb request from the given HTTP request.
func unpackProvisionRequest(r *http.Request) (*osb.ProvisionRequest, error) {
	// unpacking an osb request from an http request involves:
	// - unmarshaling the request body
	// - getting IDs out of mux vars
	// - getting query parameters from request URL
	osbRequest := &osb.ProvisionRequest{}
	if err := unmarshalRequestBody(r, osbRequest); err != nil {
		return nil, err
	}

	vars := mux.Vars(r)
	osbRequest.InstanceID = vars[osb.VarKeyInstanceID]

	asyncQueryParamVal := r.URL.Query().Get(osb.AcceptsIncomplete)
	if strings.ToLower(asyncQueryParamVal) == "true" {
		osbRequest.AcceptsIncomplete = true
	}

	return osbRequest, nil
}

// DeprovisionHandler is the mux handler that dispatches deprovision requests to
// the broker's BusinessLogic.
func (s *APISurface) DeprovisionHandler(w http.ResponseWriter, r *http.Request) {
	s.Metrics.Actions.WithLabelValues("deprovision").Inc()

	version := getBrokerAPIVersionFromRequest(r)
	if err := s.BusinessLogic.ValidateBrokerAPIVersion(version); err != nil {
		writeError(w, err, http.StatusPreconditionFailed)
		return
	}

	request, err := unpackDeprovisionRequest(r)
	if err != nil {
		writeError(w, err, http.StatusInternalServerError)
		return
	}

	glog.Infof("Received DeprovisionRequest for instanceID %q", request.InstanceID)

	c := &broker.RequestContext{
		Writer:  w,
		Request: r,
	}

	response, err := s.BusinessLogic.Deprovision(request, c)
	if err != nil {
		writeError(w, err, http.StatusInternalServerError)
		return
	}

	status := http.StatusOK
	if response.Async {
		status = http.StatusAccepted
	}

	writeResponse(w, status, response)
}

// unpackDeprovisionRequest unpacks an osb request from the given HTTP request.
func unpackDeprovisionRequest(r *http.Request) (*osb.DeprovisionRequest, error) {
	osbRequest := &osb.DeprovisionRequest{}

	vars := mux.Vars(r)
	osbRequest.InstanceID = vars[osb.VarKeyInstanceID]
	osbRequest.ServiceID = vars[osb.VarKeyServiceID]
	osbRequest.PlanID = vars[osb.VarKeyPlanID]

	asyncQueryParamVal := r.URL.Query().Get(osb.AcceptsIncomplete)
	if strings.ToLower(asyncQueryParamVal) == "true" {
		osbRequest.AcceptsIncomplete = true
	}

	return osbRequest, nil
}

// LastOperationHandler is the mux handler that dispatches last-operation
// requests to the broker's BusinessLogic.
func (s *APISurface) LastOperationHandler(w http.ResponseWriter, r *http.Request) {
	s.Metrics.Actions.WithLabelValues("last_operation").Inc()

	version := getBrokerAPIVersionFromRequest(r)
	if err := s.BusinessLogic.ValidateBrokerAPIVersion(version); err != nil {
		writeError(w, err, http.StatusPreconditionFailed)
		return
	}

	request, err := unpackLastOperationRequest(r)
	if err != nil {
		// TODO: This should return a 400 in this case as it is either
		// malformed or missing mandatory data, as per the OSB spec.
		writeError(w, err, http.StatusInternalServerError)
		return
	}

	glog.Infof("Received LastOperationRequest for instanceID %q", request.InstanceID)

	c := &broker.RequestContext{
		Writer:  w,
		Request: r,
	}

	response, err := s.BusinessLogic.LastOperation(request, c)
	if err != nil {
		// TODO: This should return a 400 in this case as it is either
		// malformed or missing mandatory data, as per the OSB spec.
		writeError(w, err, http.StatusInternalServerError)
		return
	}

	writeResponse(w, http.StatusOK, response)
}

// unpackLastOperationRequest unpacks an osb request from the given HTTP request.
func unpackLastOperationRequest(r *http.Request) (*osb.LastOperationRequest, error) {
	osbRequest := &osb.LastOperationRequest{}

	vars := mux.Vars(r)
	osbRequest.InstanceID = vars[osb.VarKeyInstanceID]
	serviceID := vars[osb.VarKeyServiceID]
	if serviceID != "" {
		osbRequest.ServiceID = &serviceID
	}
	planID := vars[osb.VarKeyPlanID]
	if planID != "" {
		osbRequest.PlanID = &planID
	}
	operation := vars[osb.VarKeyOperation]
	if operation != "" {
		typedOperation := osb.OperationKey(operation)
		osbRequest.OperationKey = &typedOperation
	}
	return osbRequest, nil
}

// BindHandler is the mux handler that dispatches bind requests to the broker's
// BusinessLogic.
func (s *APISurface) BindHandler(w http.ResponseWriter, r *http.Request) {
	s.Metrics.Actions.WithLabelValues("bind").Inc()

	version := getBrokerAPIVersionFromRequest(r)
	if err := s.BusinessLogic.ValidateBrokerAPIVersion(version); err != nil {
		writeError(w, err, http.StatusPreconditionFailed)
		return
	}

	request, err := unpackBindRequest(r)
	if err != nil {
		writeError(w, err, http.StatusInternalServerError)
		return
	}

	glog.Infof("Received BindRequest for instanceID %q, bindingID %q", request.InstanceID, request.BindingID)

	c := &broker.RequestContext{
		Writer:  w,
		Request: r,
	}

	response, err := s.BusinessLogic.Bind(request, c)
	if err != nil {
		writeError(w, err, http.StatusInternalServerError)
		return
	}

	writeResponse(w, http.StatusOK, response)
}

// unpackBindRequest unpacks an osb request from the given HTTP request.
func unpackBindRequest(r *http.Request) (*osb.BindRequest, error) {
	osbRequest := &osb.BindRequest{}
	if err := unmarshalRequestBody(r, osbRequest); err != nil {
		return nil, err
	}

	vars := mux.Vars(r)
	osbRequest.InstanceID = vars[osb.VarKeyInstanceID]
	osbRequest.BindingID = vars[osb.VarKeyBindingID]

	return osbRequest, nil
}

// UnbindHandler is the mux handler that dispatches unbind requests to the
// broker's BusinessLogic.
func (s *APISurface) UnbindHandler(w http.ResponseWriter, r *http.Request) {
	s.Metrics.Actions.WithLabelValues("unbind").Inc()

	version := getBrokerAPIVersionFromRequest(r)
	if err := s.BusinessLogic.ValidateBrokerAPIVersion(version); err != nil {
		writeError(w, err, http.StatusPreconditionFailed)
		return
	}

	request, err := unpackUnbindRequest(r)
	if err != nil {
		writeError(w, err, http.StatusInternalServerError)
		return
	}

	glog.Infof("Received UnbindRequest for instanceID %q, bindingID %q", request.InstanceID, request.BindingID)

	c := &broker.RequestContext{
		Writer:  w,
		Request: r,
	}

	response, err := s.BusinessLogic.Unbind(request, c)
	if err != nil {
		writeError(w, err, http.StatusInternalServerError)
		return
	}

	writeResponse(w, http.StatusOK, response)
}

// unpackUnbindRequest unpacks an osb request from the given HTTP request.
func unpackUnbindRequest(r *http.Request) (*osb.UnbindRequest, error) {
	osbRequest := &osb.UnbindRequest{}

	vars := mux.Vars(r)
	osbRequest.InstanceID = vars[osb.VarKeyInstanceID]
	osbRequest.BindingID = vars[osb.VarKeyBindingID]

	return osbRequest, nil
}

// UpdateHandler is the mux handler that dispatches Update requests to the
// broker's BusinessLogic.
func (s *APISurface) UpdateHandler(w http.ResponseWriter, r *http.Request) {
	s.Metrics.Actions.WithLabelValues("update").Inc()

	version := getBrokerAPIVersionFromRequest(r)
	if err := s.BusinessLogic.ValidateBrokerAPIVersion(version); err != nil {
		writeError(w, err, http.StatusPreconditionFailed)
		return
	}

	request, err := unpackUpdateRequest(r)
	if err != nil {
		writeError(w, err, http.StatusInternalServerError)
		return
	}

	glog.Infof("Received Update Request for instanceID %q", request.InstanceID)

	c := &broker.RequestContext{
		Writer:  w,
		Request: r,
	}

	response, err := s.BusinessLogic.Update(request, c)
	if err != nil {
		writeError(w, err, http.StatusInternalServerError)
		return
	}

	status := http.StatusOK
	if response.Async {
		status = http.StatusAccepted
	}

	writeResponse(w, status, response)
}

func unpackUpdateRequest(r *http.Request) (*osb.UpdateInstanceRequest, error) {
	osbRequest := &osb.UpdateInstanceRequest{}

	vars := mux.Vars(r)
	osbRequest.ServiceID = vars[osb.VarKeyServiceID]

	planID := vars[osb.VarKeyPlanID]
	if planID != "" {
		osbRequest.PlanID = &planID
	}

	return osbRequest, nil
}
