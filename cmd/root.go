/*
Copyright © 2022 Joe Searcy <joe@twr.io>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"twr.dev/cm2http/pkg/kube"
)

const (
	POD_ENV     = "CM2HTTP_POD_NAME"
	CLUSTER_ENV = "CM2HTTP_CLUSTER_NAME"
)

var (
	cmClient       *cmConfig
	cfgFile        string
	cmName         string
	cmNamespace    string
	cmKey          string
	kubeconfig     string
	kubeContext    string
	logLevel       string
	cmOptions      metav1.ListOptions
	defaultCMValue map[string]string
)

type cmConfig struct {
	k8sInterface kubernetes.Interface
	mutex        *sync.Mutex
	data         map[string]string
}

type infoResponse struct {
	Pod      string    `json:"pod"`
	Cluster  string    `json:"cluster"`
	Datetime time.Time `json:"datetime"`
}

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "cm2http",
	Short: "A utility to discover and serve the data from a Kubernetes configMap via HTTP",
	Long:  `A utility to discover and serve the data from a Kubernetes configMap via HTTP`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	Run: func(cmd *cobra.Command, args []string) {

		err := validateFlagOptions(cmd)
		if err != nil {
			fmt.Printf("Error parsing flag input: %v", err)
		}

		// Set default value for cert
		defaultCMValue = make(map[string]string)

		// Setup the Kubernetes Client
		client, err := kube.CreateKubeClient(kubeconfig, kubeContext)
		if err != nil {
			message := fmt.Sprintf("ERROR: Unable to generate kubernetes client: %v\n", err)
			panic(message)
		}

		// Setup initial info
		cmClient = &cmConfig{}
		cmClient.k8sInterface = client
		cmClient.mutex = &sync.Mutex{}

		// Setup configmap watcher
		go watchConfigMap(cmClient, cmd)

		// Handel routes
		http.HandleFunc("/info", infoRouteHandler)
		http.HandleFunc("/healthz", healthzRouteHandler)
		http.HandleFunc("/readyz", healthzRouteHandler)
		http.HandleFunc("/data", cmDataRouteHandler)

		fmt.Printf("Listening on port 5555\n")
		http.ListenAndServe(":5555", nil)

	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.cm2http.yaml)")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	//rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
	rootCmd.Flags().StringVar(&cmName, "configmap-name", "kube-root-ca.crt", "name of a configmap")
	rootCmd.Flags().StringVar(&cmNamespace, "configmap-namespace", "", "name of the namespace where the configmap is located")
	rootCmd.Flags().StringVar(&cmKey, "configmap-key", "", "name of a specific key in the configmap")
	rootCmd.Flags().StringVar(&kubeconfig, "kubeconfig", "", "name of the kubeconfig file to use. Leave blank for default/in-cluster")
	rootCmd.Flags().StringVar(&kubeContext, "context", "", "name of the kubeconfig context to use. Leave blank for default")
	rootCmd.Flags().StringVar(&logLevel, "log-level", "info", "logging level. One of \"info\" or \"debug\"")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		// Search config in home directory with name ".cm2http" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigType("yaml")
		viper.SetConfigName(".cm2http")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	}
}

// cmDataRouteHandler handles calls for the "/data" route
// This route reads the data key(s) from a configMap and outputs the data in JSON format
func cmDataRouteHandler(w http.ResponseWriter, req *http.Request) {

	// Print current CA Cert
	cmClient.mutex.Lock()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(cmClient.data)
	cmClient.mutex.Unlock()

	fmt.Printf("%q\tendpoint called [ Method: %q, Protocol: %q, User Agent: %q, Namespace: %q, ConfigMap: %q, Key: %q ]\n", req.RequestURI, req.Method, req.Proto, req.Header.Get("User-Agent"), cmNamespace, cmName, cmKey)
}

// infoRouteHandler handles calls for the "/info" route
// This route ouputs info about the environment
func infoRouteHandler(w http.ResponseWriter, req *http.Request) {

	var responseInfo infoResponse

	// Set Response Body
	cmClient.mutex.Lock()
	responseInfo.Cluster = os.Getenv(CLUSTER_ENV)
	responseInfo.Pod = os.Getenv(POD_ENV)
	responseInfo.Datetime = time.Now()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(responseInfo)
	cmClient.mutex.Unlock()

	fmt.Printf("%q\tendpoint called [ Method: %q, Protocol: %q, User Agent: %q, Namespace: %q, ConfigMap: %q, Key: %q ]\n", req.RequestURI, req.Method, req.Proto, req.Header.Get("User-Agent"), cmNamespace, cmName, cmKey)
}

// healthzRouteHandler/readyzRouteHandler handles calls for the "/healthz" and "/readyz" routes
// This route outputs the current health/ready status of the app
func healthzRouteHandler(w http.ResponseWriter, req *http.Request) {

	response := make(map[string]string)

	// Set route type based on whether it's called as "/readyz" or "/healthz"
	routeType := "healthy"
	if req.RequestURI == "/readyz" {
		routeType = "ready"
	}

	response[routeType] = "true"

	// Set Response Body
	cmClient.mutex.Lock()
	w.Header().Set("Content-Type", "application/json")
	jsonResponse, err := json.Marshal(response)
	if err != nil {
		panic("Unable to marshal response body to JSON" + err.Error())
	}
	w.Write(jsonResponse)
	cmClient.mutex.Unlock()

	// Only log calls to "/healthz" and "/readyz" if debug log-level is selected
	if strings.ToLower(logLevel) == "debug" {
		fmt.Printf("%q\tendpoint called [ Method: %q, Protocol: %q, User Agent: %q, Namespace: %q, ConfigMap: %q, Key: %q ]\n", req.RequestURI, req.Method, req.Proto, req.Header.Get("User-Agent"), cmNamespace, cmName, cmKey)
	}
}

// watchConfigMap to stand up a watcher for the configMap
func watchConfigMap(cmClient *cmConfig, cmd *cobra.Command) {

	// Set options to filter for a single configMap object
	cmOptions = metav1.SingleObject(metav1.ObjectMeta{Name: cmName, Namespace: cmNamespace})

	// Watch for events on configMap
	for {
		watcher, err := cmClient.k8sInterface.CoreV1().ConfigMaps(cmNamespace).Watch(context.TODO(), cmOptions)
		if err != nil {
			panic("Unable to create watcher: " + err.Error())
		}

		// Update Serviced Data
		updateCMData(watcher.ResultChan(), cmClient, cmd)
	}
}

// updateCMData updates the data served upon configMap changes
func updateCMData(eventChannel <-chan watch.Event, cmClient *cmConfig, cmd *cobra.Command) {
	// React to incoming events on the channel
	for {
		event, open := <-eventChannel

		if open {

			// Parse based on incoming event type
			switch event.Type {

			// Handle Object added
			case watch.Added:

				fallthrough

			// Handle object modified
			case watch.Modified:

				fmt.Printf("Target configmap \"%v/%v\" has been modified\n", cmNamespace, cmName)

				// Update the CM Data
				cmClient.mutex.Lock()
				if cm, ok := event.Object.(*corev1.ConfigMap); ok {
					fmt.Printf("Object retrieved from watcher is of Kind ConfigMap\n")
					if cmd.Flag("configmap-key").Changed {

						if cmValue, ok := cm.Data[cmKey]; ok {
							fmt.Printf("%q configMap key specified/or using default, serving single key", cmKey)
							fmt.Printf("Object retrieved from watcher has target data key %q\n", cmKey)
							tmpData := make(map[string]string)
							tmpData[cmKey] = cmValue
							cmClient.data = tmpData
							fmt.Printf("Updating Data Served\n")
						} else {
							fmt.Printf("Key not found in configMap. Serving default value\n")
						}
					} else if len(cm.Data) >= 1 {
						fmt.Printf("No configMap key specified, serving all data keys")
						cmClient.data = cm.Data
						fmt.Printf("Updating Data Served\n")
					} else {
						fmt.Printf("ConfigMap has no Data Keys. Serving default value\n")
					}
				} else {
					fmt.Printf("Object retrieved from watcher is not a ConfigMap")
				}
				cmClient.mutex.Unlock()

			// Handle object deleted
			case watch.Deleted:

				fmt.Printf("Target configmap \"%v/%v\" has been deleted\n", cmNamespace, cmName)

				// Fall back to the default value
				cmClient.mutex.Lock()
				cmClient.data = defaultCMValue
				fmt.Printf("Setting default value: %v\n", cmClient.data)
				cmClient.mutex.Unlock()

			default:
				// Do nothing
			}
		} else {
			// If eventChannel is closed, it means the server has closed the connection
			return
		}
	}
}

func validateFlagOptions(cmd *cobra.Command) error {
	if strings.ToLower(cmd.Flag("log-level").Value.String()) != "info" && strings.ToLower(cmd.Flag("log-level").Value.String()) != "debug" {
		errString := fmt.Sprintf("option %q passed to %q flag is not valid. Using default value %q\n", cmd.Flag("log-level").Value.String(), cmd.Flag("log-level").Name, cmd.Flag("log-level").DefValue)
		return errors.New(errString)
	}

	return nil
}
