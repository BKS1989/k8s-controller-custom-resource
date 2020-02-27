package main

import (
	"k8s.io/client-go/kubernetes"
	samplecrdv1 "github.com/bks1989/k8s-controller-custom-resource/pkg/apis/samplecrd/v1"
	clientset "github.com/bks1989/k8s-controller-custom-resource/pkg/client/clientset/versioned"
	networkscheme "github.com/bks1989/k8s-controller-custom-resource/pkg/client/clientset/versioned/scheme"
	informers "github.com/bks1989/k8s-controller-custom-resource/pkg/client/informers/externalversions/samplecrd/v1"
	listers "github.com/bks1989/k8s-controller-custom-resource/pkg/client/listers/samplecrd/v1"
)

const controllerAgentName = "network-controller"

const (
	SuccessSynced         = true
	MessageResourceSynced = "Network sync successfully"
)

type Controller struct {
	kubeclientset kubernetes.interface
	networkclientset clientset.int
}