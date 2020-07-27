package installation

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"

	"github.com/kyma-incubator/hydroform/install/installation"
	"github.com/kyma-project/kyma/components/kyma-operator/pkg/apis/installer/v1alpha1"
	"k8s.io/client-go/rest"
)

const (
	tillerWaitTime = 10 * time.Minute
	installAction  = "installation"
	upgradeAction  = "upgrade"
)

//go:generate mockery --name Service

type Service interface {
	CheckInstallationState(kubeconfig *rest.Config) (installation.InstallationState, error)
	TriggerInstallation(tillerYaml string, installerYaml string, installerCRYaml string, configuration installation.Configuration) error
	TriggerUpgrade(tillerYaml string, installerYaml string, installerCRYaml string, configuration installation.Configuration) error
	TriggerUninstall(kubeconfig *rest.Config) error
}

func NewInstallationService(kubeconfig *rest.Config, installationTimeout time.Duration, clusterCleanupResourceSelector string, componentsConfig []v1alpha1.KymaComponent) (Service, error) {
	opts := []installation.InstallationOption{installation.WithTillerWaitTime(tillerWaitTime)}
	if len(componentsConfig) > 0 {
		opts = append(opts, installation.WithInstallationCRModification(GetInstallationCRModificationFunc(componentsConfig)))
	}
	installer, err := installation.NewKymaInstaller(kubeconfig, opts...)
	if err != nil {
		return nil, err
	}

	return &installationService{
		kymaInstallationTimeout:        installationTimeout,
		kymaInstaller:                  *installer,
		clusterCleanupResourceSelector: clusterCleanupResourceSelector,
	}, nil
}

type installationService struct {
	kymaInstallationTimeout        time.Duration
	kymaInstaller                  installation.Installer
	clusterCleanupResourceSelector string
}

func (s *installationService) TriggerInstallation(tillerYaml string, installerYaml string, installerCRYaml string, configuration installation.Configuration) error {
	return s.triggerAction(tillerYaml, installerYaml, installerCRYaml, configuration, s.kymaInstaller.PrepareInstallation, installAction)
}

func (s *installationService) TriggerUpgrade(tillerYaml string, installerYaml string, installerCRYaml string, configuration installation.Configuration) error {
	return s.triggerAction(tillerYaml, installerYaml, installerCRYaml, configuration, s.kymaInstaller.PrepareUpgrade, upgradeAction)
}

func (s *installationService) triggerAction(
	tillerYaml string,
	installerYaml string,
	installerCRYaml string,
	configuration installation.Configuration,
	prepareFunction func(installation.Installation) error,
	actionName string) error {

	installationConfig := installation.Installation{
		TillerYaml:      tillerYaml,
		InstallerYaml:   installerYaml,
		InstallerCRYaml: installerCRYaml,
		Configuration:   configuration,
	}

	err := prepareFunction(installationConfig)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("Failed to prepare %s", actionName))
	}

	installationCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// We are not waiting for events, just triggering installation
	_, _, err = s.kymaInstaller.StartInstallation(installationCtx)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("Failed to start Kyma %s", actionName))
	}

	return nil
}

func (s *installationService) CheckInstallationState(kubeconfig *rest.Config) (installation.InstallationState, error) {
	return installation.CheckInstallationState(kubeconfig)
}

func (s *installationService) TriggerUninstall(kubeconfig *rest.Config) error {
	return installation.TriggerUninstall(kubeconfig)
}

func GetInstallationCRModificationFunc(componentsList []v1alpha1.KymaComponent) func(*v1alpha1.Installation) {
	return func(installation *v1alpha1.Installation) {
		if len(componentsList) > 0 {
			installation.Spec.Components = componentsList
		}
	}
}
