// Copyright (c) 2022 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package util

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"k8s.io/client-go/tools/clientcmd/api"
)

var (
	errContextNotFound  = errors.New("context not found")
	errClusterNotFound  = errors.New("cluster not found")
	errAuthInfoNotFound = errors.New("user not found")
	errUnknownURLFormat = errors.New("Unknown format for server URL")
	errNoToken          = errors.New("No Token found")
)

// GetNonK8sAPIURL returns the URL of Non-K8s API
func GetNonK8sAPIURL(config api.Config) (string, error) {
	serverURLString, err := getServerURL(config)
	if err != nil {
		return "", fmt.Errorf("Server URL not found: %w", err)
	}

	serverURL, err := url.Parse(serverURLString)
	if err != nil {
		return "", fmt.Errorf("Unable to parse server URL %s: %w", serverURL, err)
	}

	hostWithoutPort := strings.Split(serverURL.Host, ":")[0]

	nonK8sAPIURL := strings.TrimPrefix(hostWithoutPort, "api.")
	if nonK8sAPIURL == "" {
		return "", fmt.Errorf("%w: for %s", errUnknownURLFormat, hostWithoutPort)
	}

	return nonK8sAPIURL, nil
}

func getServerURL(config api.Config) (string, error) {
	currentContext, found := config.Contexts[config.CurrentContext]
	if !found {
		return "", fmt.Errorf("%w: for %s", errContextNotFound, config.CurrentContext)
	}

	currentCluster, found := config.Clusters[currentContext.Cluster]
	if !found {
		return "", fmt.Errorf("%w: for %s", errClusterNotFound, currentContext.Cluster)
	}

	return currentCluster.Server, nil
}

// GetToken returns the token (if token-authentication is used) from kube config
func GetToken(config api.Config) (string, error) {
	currentContext, found := config.Contexts[config.CurrentContext]
	if !found {
		return "", fmt.Errorf("%w: for %s", errContextNotFound, config.CurrentContext)
	}

	currentAuthInfo, found := config.AuthInfos[currentContext.AuthInfo]
	if !found {
		return "", fmt.Errorf("%w: for %s", errAuthInfoNotFound, currentContext.AuthInfo)
	}

	if currentAuthInfo.Token == "" {
		return "", fmt.Errorf("%w: for %s", errNoToken, currentContext.AuthInfo)
	}

	return currentAuthInfo.Token, nil
}
