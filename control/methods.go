package control

import (
	"errors"
	"net/http"
	"runtime"

	interrs "ratelimiter/internal/errors"
	"ratelimiter/service"
)

type ControlMethods struct {
	manager *service.Manager
}

func NewControlMethods(manager *service.Manager) *ControlMethods {
	cm := &ControlMethods{
		manager: manager,
	}
	return cm
}

func (s *ControlMethods) EnableProfiling(r *http.Request) (any, error) {
	runtime.SetBlockProfileRate(5)
	runtime.SetMutexProfileFraction(5)
	return true, nil
}

func (s *ControlMethods) DisableProfiling(r *http.Request) (any, error) {
	runtime.SetBlockProfileRate(0)
	runtime.SetMutexProfileFraction(0)
	return true, nil
}

func (s *ControlMethods) GetStat(r *http.Request) (any, error) {
	stat := s.manager.GetStat()
	return stat, nil
}

func (s *ControlMethods) GetInfo(r *http.Request) (any, error) {
	stat := s.manager.GetInfo()
	return stat, nil
}

func (s *ControlMethods) AddGroup(r *http.Request) (any, error) {
	req, err := DecodeRequest[addGroupRequest](r.Body)
	if err != nil {
		return nil, NewValidationError("incorrect data", err)
	}

	err = s.manager.AddGroup(req.Name, float64(req.Rate), int(req.Burst))
	if err != nil {
		if errors.Is(err, interrs.ErrInvalidData) {
			return nil, NewValidationError(err.Error(), err)
		}

		// errors are split into 2 groups:
		// 1) known errors that we handle and show to user in a clear way
		if errors.Is(err, service.ErrGroupExists) {
			// user sees "group already exists"; veriable err stays in logs
			return nil, NewLogicError("group already exists", err, http.StatusConflict)
		}

		// 2) unknown errors can happen deeper in code (in third-party libs for example)
		// just pass them through and let upper layer decide

		return nil, err
	}
	return true, nil
}

func (s *ControlMethods) DeleteGroup(r *http.Request) (any, error) {
	req, err := DecodeRequest[deleteGroupRequest](r.Body)
	if err != nil {
		return nil, NewValidationError("incorrect data", err)
	}

	s.manager.DeleteGroup(req.Name)
	return true, nil
}

func (s *ControlMethods) UpdateGroup(r *http.Request) (any, error) {
	req, err := DecodeRequest[updateGroupRequest](r.Body)
	if err != nil {
		return nil, NewValidationError("incorrect data", err)
	}

	err = s.manager.UpdateGroup(req.Name, float64(req.Rate), int(req.Burst))
	if err != nil {

		if errors.Is(err, interrs.ErrInvalidData) {
			return nil, NewValidationError(err.Error(), err)
		}

		if errors.Is(err, service.ErrGroupNotFound) {
			return nil, NewNotFoundError("group not found", err)
		}

		return nil, err
	}
	return true, nil
}
