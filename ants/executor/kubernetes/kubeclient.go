package kubernetes

import (
	"fmt"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"net/http"
	config2 "plexobject.com/formicary/internal/ant_config"
)

// getKubeClient creates kubernetes client with enhanced configuration support
func getKubeClient(config *config2.AntConfig) (cli *kubernetes.Clientset, restConfig *restclient.Config, err error) {
	//if ant_config.Kubernetes.Host == "" {
	//	restConfig, err = guessClientConfig()
	//} else {
	//	restConfig, err = getOutClusterClientConfig(&ant_config.Kubernetes)
	//}
	//if err != nil {
	//	return nil, nil, err
	//}
	//cli, err = kubernetes.NewForConfig(restConfig)
	// Initialize the enhanced kubernetes client
	if err = config.Kubernetes.InitializeKubernetesClient(); err != nil {
		return nil, nil, fmt.Errorf("failed to initialize kubernetes client: %w", err)
	}

	// Get the initialized client
	cli, ok := config.Kubernetes.GetClient().(*kubernetes.Clientset)
	if !ok || cli == nil {
		return nil, nil, fmt.Errorf("failed to get kubernetes clientset")
	}

	// Get the REST config (for backward compatibility with existing code)
	restConfig = config.Kubernetes.SelectedKubeconfig
	if restConfig == nil {
		return nil, nil, fmt.Errorf("no REST config available")
	}

	return cli, restConfig, nil
}

func getOutClusterClientConfig(config *config2.KubernetesConfig) (*restclient.Config, error) {
	return config.GetKubeConfigForCluster()
}

// Deprecated: Use KubernetesConfig.InitializeKubernetesClient() instead
func guessClientConfig() (*restclient.Config, error) {
	// Try in cluster config first
	if inClusterCfg, err := restclient.InClusterConfig(); err == nil {
		return inClusterCfg, nil
	}

	// in cluster config failed. Reading default kubectl config
	return _defaultKubectlConfig()
}

// See https://godoc.org/k8s.io/client-go/tools/clientcmd#ClientConfigLoadingRules
// https://godoc.org/k8s.io/client-go/tools/clientcmd/api#Config
// Deprecated: Use KubernetesConfig.InitializeKubernetesClient() instead
func _defaultKubectlConfig() (*restclient.Config, error) {
	// new implementation Create a temporary config to use the enhanced logic
	//tempConfig := &config.KubernetesConfig{}
	//return tempConfig.getKubeConfigForCluster()
	// old implementation below
	load, err := clientcmd.NewDefaultClientConfigLoadingRules().Load()
	if err != nil {
		return nil, err
	}

	return clientcmd.NewDefaultClientConfig(*load, &clientcmd.ConfigOverrides{}).ClientConfig()
}

func closeKubeClient(client *kubernetes.Clientset) bool {
	if client == nil {
		return false
	}
	rest, ok := client.CoreV1().RESTClient().(*restclient.RESTClient)
	if !ok || rest.Client == nil || rest.Client.Transport == nil {
		return false
	}
	if transport, ok := rest.Client.Transport.(*http.Transport); ok {
		transport.CloseIdleConnections()
		return true
	}
	return false
}
