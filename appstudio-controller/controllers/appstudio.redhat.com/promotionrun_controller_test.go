package appstudioredhatcom

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appstudiosharedv1 "github.com/redhat-appstudio/managed-gitops/appstudio-shared/apis/appstudio.redhat.com/v1alpha1"
	apibackend "github.com/redhat-appstudio/managed-gitops/backend-shared/apis/managed-gitops/v1alpha1"
	"github.com/redhat-appstudio/managed-gitops/backend-shared/util/tests"
)

var _ = Describe("SnapshotEnvironmentBinding Reconciler Tests", func() {
	Context("Testing SnapshotEnvironmentBindingReconciler.", func() {

		var ctx context.Context
		var request reconcile.Request
		var environment appstudiosharedv1.Environment
		var promotionRun *appstudiosharedv1.PromotionRun
		var promotionRunReconciler PromotionRunReconciler

		BeforeEach(func() {
			ctx = context.Background()

			scheme,
				argocdNamespace,
				kubesystemNamespace,
				apiNamespace,
				err := tests.GenericTestSetup()
			Expect(err).To(BeNil())

			err = appstudiosharedv1.AddToScheme(scheme)
			Expect(err).To(BeNil())

			By("Create fake client.")
			k8sClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(apiNamespace, argocdNamespace, kubesystemNamespace).
				Build()

			By("Create placeholder environment.")
			environment = appstudiosharedv1.Environment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "prod",
					Namespace: apiNamespace.Name,
				},
				Spec: appstudiosharedv1.EnvironmentSpec{
					DisplayName:        "my-environment",
					Type:               appstudiosharedv1.EnvironmentType_POC,
					DeploymentStrategy: appstudiosharedv1.DeploymentStrategy_AppStudioAutomated,
					ParentEnvironment:  "",
					Tags:               []string{},
					Configuration:      appstudiosharedv1.EnvironmentConfiguration{},
				},
			}
			err = k8sClient.Create(ctx, &environment)
			Expect(err).To(BeNil())

			promotionRunReconciler = PromotionRunReconciler{Client: k8sClient, Scheme: scheme}

			promotionRun = &appstudiosharedv1.PromotionRun{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "new-demo-app-manual-promotion",
					Namespace: apiNamespace.Name,
				},
				Spec: appstudiosharedv1.PromotionRunSpec{
					Snapshot:    "my-snapshot",
					Application: "new-demo-app",
					ManualPromotion: appstudiosharedv1.ManualPromotionConfiguration{
						TargetEnvironment: "prod",
					},
				},
			}

			request = newRequest(apiNamespace.Name, promotionRun.Name)
		})

		It("Should do nothing as PromotionRun CR is not created.", func() {
			By("Trigger Reconciler.")
			_, err := promotionRunReconciler.Reconcile(ctx, request)
			Expect(err).To(BeNil())
		})

		It("Should do nothing as status is complete.", func() {
			By("setting the Status.State to Complete.")
			promotionRun.Status = appstudiosharedv1.PromotionRunStatus{
				State: appstudiosharedv1.PromotionRunState_Complete,
			}
			err := promotionRunReconciler.Create(ctx, promotionRun)
			Expect(err).To(BeNil())

			By("Trigger Reconciler.")
			_, err = promotionRunReconciler.Reconcile(ctx, request)
			Expect(err).To(BeNil())

		})

		It("Should fetch other PromotionRun CRs and ignore completed CRs.", func() {
			err := promotionRunReconciler.Create(ctx, promotionRun)
			Expect(err).To(BeNil())

			By("Create another PromotionRun CR.")
			promotionRunTemp := &appstudiosharedv1.PromotionRun{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "new-demo-app-auto-promotion",
					Namespace: promotionRun.Namespace,
				},
				Spec: appstudiosharedv1.PromotionRunSpec{
					Snapshot:    "my-snapshot",
					Application: "new-demo-app",
					ManualPromotion: appstudiosharedv1.ManualPromotionConfiguration{
						TargetEnvironment: "prod",
					},
				},
				Status: appstudiosharedv1.PromotionRunStatus{
					State: appstudiosharedv1.PromotionRunState_Complete,
				},
			}

			err = promotionRunReconciler.Create(ctx, promotionRunTemp)
			Expect(err).To(BeNil())

			By("Trigger Reconciler.")
			_, err = promotionRunReconciler.Reconcile(ctx, request)
			Expect(err).To(BeNil())
		})

		It("Should fetch other PromotionRun CRs and ignore if Spec.Application is different.", func() {
			err := promotionRunReconciler.Create(ctx, promotionRun)
			Expect(err).To(BeNil())

			By("Create another PromotionRun CR.")
			promotionRunTemp := &appstudiosharedv1.PromotionRun{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "new-demo-app-auto-promotion",
					Namespace: promotionRun.Namespace,
				},
				Spec: appstudiosharedv1.PromotionRunSpec{
					Snapshot:    "my-snapshot",
					Application: "new-demo-app-v1",
					ManualPromotion: appstudiosharedv1.ManualPromotionConfiguration{
						TargetEnvironment: "prod",
					},
				},
			}

			err = promotionRunReconciler.Create(ctx, promotionRunTemp)
			Expect(err).To(BeNil())

			By("Trigger Reconciler.")
			_, err = promotionRunReconciler.Reconcile(ctx, request)
			Expect(err).To(BeNil())
		})

		It("Should return error if another PromotionRun CR is available pointing to same Application.", func() {
			promotionRun.CreationTimestamp = metav1.NewTime(time.Now().Add(-time.Minute * 5))
			err := promotionRunReconciler.Create(ctx, promotionRun)
			Expect(err).To(BeNil())

			By("Create another PromotionRun CR.")
			promotionRunTemp := &appstudiosharedv1.PromotionRun{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "new-demo-app-auto-promotion",
					Namespace:         promotionRun.Namespace,
					CreationTimestamp: metav1.NewTime(time.Date(2022, 7, 20, 20, 34, 58, 651387237, time.UTC)),
				},
				Spec: appstudiosharedv1.PromotionRunSpec{
					Snapshot:    "my-snapshot",
					Application: "new-demo-app",
					ManualPromotion: appstudiosharedv1.ManualPromotionConfiguration{
						TargetEnvironment: "prod",
					},
				},
			}

			err = promotionRunReconciler.Create(ctx, promotionRunTemp)
			Expect(err).To(BeNil())

			By("Trigger Reconciler.")
			_, err = promotionRunReconciler.Reconcile(ctx, request)
			Expect(err).To(BeNil())
			checkStatusCondition(ctx, promotionRunReconciler.Client, promotionRun, "Error occurred while checking for existing active promotions.")
		})

		It("Should not support auto Promotion and Status.Condition should be updated if it already exists.", func() {
			promotionRun.Spec.AutomatedPromotion.InitialEnvironment = "abc"
			err := promotionRunReconciler.Create(ctx, promotionRun)
			Expect(err).To(BeNil())

			By("Trigger Reconciler.")
			_, err = promotionRunReconciler.Reconcile(ctx, request)
			Expect(err).To(BeNil())

			err = promotionRunReconciler.Client.Get(ctx, client.ObjectKeyFromObject(promotionRun), promotionRun)
			Expect(err).To(BeNil())
			conditionsLen := len(promotionRun.Status.Conditions)
			Expect(conditionsLen > 0)
			checkStatusCondition(ctx, promotionRunReconciler.Client, promotionRun, "Automated promotion are not yet supported.")

			By("Trigger Reconciler again.")
			_, err = promotionRunReconciler.Reconcile(ctx, request)
			Expect(err).To(BeNil())

			By("Check existing condition is updated instead of creating new.")
			err = promotionRunReconciler.Client.Get(ctx, client.ObjectKeyFromObject(promotionRun), promotionRun)
			Expect(err).To(BeNil())
			Expect(conditionsLen > 0)
			Expect(len(promotionRun.Status.Conditions) == conditionsLen)
		})

		It("Should not support invalid value for Target Environment", func() {
			By("Set invalid Target Env.")
			promotionRun.Spec.ManualPromotion.TargetEnvironment = ""
			err := promotionRunReconciler.Create(ctx, promotionRun)
			Expect(err).To(BeNil())

			By("Trigger Reconciler.")
			_, err = promotionRunReconciler.Reconcile(ctx, request)
			Expect(err).To(BeNil())
			checkStatusCondition(ctx, promotionRunReconciler.Client, promotionRun, ErrMessageTargetEnvironmentHasInvalidValue)
		})

		It("Should do nothing as binding for application is not present.", func() {
			err := promotionRunReconciler.Create(ctx, promotionRun)
			Expect(err).To(BeNil())

			By("Trigger Reconciler.")
			_, err = promotionRunReconciler.Reconcile(ctx, request)
			Expect(err).To(BeNil())

		})

		It("PromotionRun Reconciler should successfully locate and use the Binding CR created for given PromotionRun CR.", func() {
			By("Create Snapshot CR.")
			snapshot := appstudiosharedv1.Snapshot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      promotionRun.Spec.Snapshot,
					Namespace: promotionRun.Namespace,
				},
				Spec: appstudiosharedv1.SnapshotSpec{
					Application: promotionRun.Spec.Application,
					DisplayName: promotionRun.Spec.Application,
				},
			}

			err := promotionRunReconciler.Create(ctx, &snapshot)
			Expect(err).To(BeNil())

			By("Create SnapshotEnvironmentBinding CR.")
			binding := &appstudiosharedv1.SnapshotEnvironmentBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "appa-staging-binding",
					Namespace: promotionRun.Namespace,
				},
				Spec: appstudiosharedv1.SnapshotEnvironmentBindingSpec{
					Application: promotionRun.Spec.Application,
					Environment: promotionRun.Spec.ManualPromotion.TargetEnvironment,
					Snapshot:    promotionRun.Spec.Snapshot,
					Components: []appstudiosharedv1.BindingComponent{
						{
							Name: "component-a",
							Configuration: appstudiosharedv1.BindingComponentConfiguration{
								Env: []appstudiosharedv1.EnvVarPair{
									{Name: "My_STG_ENV", Value: "1000"},
								},
								Replicas: 3,
							},
						},
					},
				},
			}

			err = promotionRunReconciler.Create(ctx, binding)
			Expect(err).To(BeNil())

			err = promotionRunReconciler.Create(ctx, promotionRun)
			Expect(err).To(BeNil())

			By("Trigger Reconciler.")
			_, err = promotionRunReconciler.Reconcile(ctx, request)
			Expect(err).To(BeNil())

			err = promotionRunReconciler.Client.Get(ctx, client.ObjectKeyFromObject(promotionRun), promotionRun)
			Expect(err).To(BeNil())
			Expect(promotionRun.Status.State).To(Equal(appstudiosharedv1.PromotionRunState_Active))
			Expect(len(promotionRun.Status.ActiveBindings)).To(Equal(1))
			Expect(promotionRun.Status.ActiveBindings[0]).To(Equal(binding.Name))
		})

		It("Should return error if PromotionRun.Status.ActiveBindings do not match with Binding given in PromotionRun CR.", func() {
			By("Create Snapshot CR.")
			snapshot := appstudiosharedv1.Snapshot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      promotionRun.Spec.Snapshot,
					Namespace: promotionRun.Namespace,
				},
				Spec: appstudiosharedv1.SnapshotSpec{
					Application: promotionRun.Spec.Application,
					DisplayName: promotionRun.Spec.Application,
				},
			}

			err := promotionRunReconciler.Create(ctx, &snapshot)
			Expect(err).To(BeNil())

			By("Create SnapshotEnvironmentBinding CR.")
			binding := &appstudiosharedv1.SnapshotEnvironmentBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "appa-staging-binding",
					Namespace: promotionRun.Namespace,
				},
				Spec: appstudiosharedv1.SnapshotEnvironmentBindingSpec{
					Application: promotionRun.Spec.Application,
					Environment: promotionRun.Spec.ManualPromotion.TargetEnvironment,
					Snapshot:    promotionRun.Spec.Snapshot,
					Components: []appstudiosharedv1.BindingComponent{
						{
							Name: "component-a",
							Configuration: appstudiosharedv1.BindingComponentConfiguration{
								Env: []appstudiosharedv1.EnvVarPair{
									{Name: "My_STG_ENV", Value: "1000"},
								},
								Replicas: 3,
							},
						},
					},
				},
			}

			err = promotionRunReconciler.Create(ctx, binding)
			Expect(err).To(BeNil())

			By("Set the ActiveBinding for Promotion CR.")
			promotionRun.Status.ActiveBindings = []string{"binding1"}

			err = promotionRunReconciler.Create(ctx, promotionRun)
			Expect(err).To(BeNil())

			By("Trigger Reconciler.")
			_, err = promotionRunReconciler.Reconcile(ctx, request)
			Expect(err).To(BeNil())
			checkStatusCondition(ctx, promotionRunReconciler.Client, promotionRun, "The binding changed after the PromotionRun first start. The .spec fields of the PromotionRun are immutable, and should not be changed after being created. old-binding: binding1, new-binding: appa-staging-binding")
		})

		It("Should return error if SnapShot given in PromotionRun CR does not exist.", func() {
			By("Create SnapshotEnvironmentBinding CR.")
			binding := &appstudiosharedv1.SnapshotEnvironmentBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "appa-staging-binding",
					Namespace: promotionRun.Namespace,
				},
				Spec: appstudiosharedv1.SnapshotEnvironmentBindingSpec{
					Application: promotionRun.Spec.Application,
					Environment: promotionRun.Spec.ManualPromotion.TargetEnvironment,
					Snapshot:    promotionRun.Spec.Snapshot,
					Components: []appstudiosharedv1.BindingComponent{
						{
							Name: "component-a",
							Configuration: appstudiosharedv1.BindingComponentConfiguration{
								Env: []appstudiosharedv1.EnvVarPair{
									{Name: "My_STG_ENV", Value: "1000"},
								},
								Replicas: 3,
							},
						},
					},
				},
			}

			err := promotionRunReconciler.Create(ctx, binding)
			Expect(err).To(BeNil())

			err = promotionRunReconciler.Create(ctx, promotionRun)
			Expect(err).To(BeNil())

			By("Trigger Reconciler.")
			_, err = promotionRunReconciler.Reconcile(ctx, request)
			Expect(err).To(BeNil())

			checkStatusCondition(ctx, promotionRunReconciler.Client, promotionRun, "Snapshot: "+promotionRun.Spec.Snapshot+" referred in Binding: "+binding.Name+" does not exist.")
		})

		It("Should wait for the environment binding to create all of the expected GitOpsDeployments.", func() {
			By("Create Snapshot CR.")
			snapshot := appstudiosharedv1.Snapshot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      promotionRun.Spec.Snapshot,
					Namespace: promotionRun.Namespace,
				},
				Spec: appstudiosharedv1.SnapshotSpec{
					Application: promotionRun.Spec.Application,
					DisplayName: promotionRun.Spec.Application,
				},
			}

			err := promotionRunReconciler.Create(ctx, &snapshot)
			Expect(err).To(BeNil())

			By("Create SnapshotEnvironmentBinding CR.")
			binding := &appstudiosharedv1.SnapshotEnvironmentBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "appa-staging-binding",
					Namespace: promotionRun.Namespace,
				},
				Spec: appstudiosharedv1.SnapshotEnvironmentBindingSpec{
					Application: promotionRun.Spec.Application,
					Environment: promotionRun.Spec.ManualPromotion.TargetEnvironment,
					Snapshot:    promotionRun.Spec.Snapshot,
					Components: []appstudiosharedv1.BindingComponent{
						{
							Name: "component-a",
							Configuration: appstudiosharedv1.BindingComponentConfiguration{
								Env: []appstudiosharedv1.EnvVarPair{
									{Name: "My_STG_ENV", Value: "1000"},
								},
								Replicas: 3,
							},
						},
					},
				},
			}

			err = promotionRunReconciler.Create(ctx, binding)
			Expect(err).To(BeNil())

			By("Set Active Binding for Promotion CR.")
			promotionRun.Status.ActiveBindings = []string{binding.Name}
			err = promotionRunReconciler.Create(ctx, promotionRun)
			Expect(err).To(BeNil())

			By("Trigger Reconciler.")
			_, err = promotionRunReconciler.Reconcile(ctx, request)
			Expect(err).To(BeNil())

			err = promotionRunReconciler.Client.Get(ctx, client.ObjectKeyFromObject(promotionRun), promotionRun)
			Expect(err).To(BeNil())
			Expect(len(promotionRun.Status.EnvironmentStatus) > 0).To(BeTrue())
			Expect(promotionRun.Status.EnvironmentStatus[0].Step).To(Equal(1))
			Expect(promotionRun.Status.EnvironmentStatus[0].DisplayStatus).To(Equal("Waiting for the environment binding to create all of the expected GitOpsDeployments."))
			Expect(promotionRun.Status.EnvironmentStatus[0].Status).To(Equal(appstudiosharedv1.PromotionRunEnvironmentStatus_InProgress))
			Expect(promotionRun.Status.EnvironmentStatus[0].EnvironmentName).To(Equal(environment.Name))
		})

		It("Should wait for GitOpsDeployments to have expected commit/sync/health: Scenario 2.", func() {
			By("Create Snapshot CR.")
			snapshot := appstudiosharedv1.Snapshot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      promotionRun.Spec.Snapshot,
					Namespace: promotionRun.Namespace,
				},
				Spec: appstudiosharedv1.SnapshotSpec{
					Application: promotionRun.Spec.Application,
					DisplayName: promotionRun.Spec.Application,
				},
			}

			err := promotionRunReconciler.Create(ctx, &snapshot)
			Expect(err).To(BeNil())

			By("Create SnapshotEnvironmentBinding CR.")
			binding := &appstudiosharedv1.SnapshotEnvironmentBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "appa-staging-binding",
					Namespace: promotionRun.Namespace,
				},
				Spec: appstudiosharedv1.SnapshotEnvironmentBindingSpec{
					Application: promotionRun.Spec.Application,
					Environment: promotionRun.Spec.ManualPromotion.TargetEnvironment,
					Snapshot:    promotionRun.Spec.Snapshot,
					Components: []appstudiosharedv1.BindingComponent{
						{
							Name: "component-a",
							Configuration: appstudiosharedv1.BindingComponentConfiguration{
								Env: []appstudiosharedv1.EnvVarPair{
									{Name: "My_STG_ENV", Value: "1000"},
								},
								Replicas: 3,
							},
						},
					},
				},
				Status: appstudiosharedv1.SnapshotEnvironmentBindingStatus{
					GitOpsDeployments: []appstudiosharedv1.BindingStatusGitOpsDeployment{
						{
							ComponentName:    "component-a",
							GitOpsDeployment: "appa-staging-binding-" + promotionRun.Spec.Application + "-" + promotionRun.Spec.ManualPromotion.TargetEnvironment + "-component-a",
						},
					},
				},
			}

			err = promotionRunReconciler.Create(ctx, binding)
			Expect(err).To(BeNil())

			By("Create GitOpsDeployment CR.")
			gitOpsDeployment := &apibackend.GitOpsDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "appa-staging-binding-" + promotionRun.Spec.Application + "-" + promotionRun.Spec.ManualPromotion.TargetEnvironment + "-component-a",
					Namespace: binding.Namespace,
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: binding.APIVersion,
							Kind:       binding.Kind,
							Name:       binding.Name,
							UID:        binding.UID,
						},
					},
				},
				Spec: apibackend.GitOpsDeploymentSpec{
					Source: apibackend.ApplicationSource{
						RepoURL:        "https://github.com/redhat-appstudio/managed-gitops",
						Path:           "resources/test-data/sample-gitops-repository/components/componentA/overlays/staging",
						TargetRevision: "main",
					},
					Type:        apibackend.GitOpsDeploymentSpecType_Automated, // Default to automated, for now
					Destination: apibackend.ApplicationDestination{},           // Default to same namespace, for now
				},
				Status: apibackend.GitOpsDeploymentStatus{
					Sync: apibackend.SyncStatus{
						Status: apibackend.SyncStatusCodeSynced,
					},
				},
			}

			err = promotionRunReconciler.Create(ctx, gitOpsDeployment)
			Expect(err).To(BeNil())

			By("Set Active Binding for Promotion.")
			promotionRun.Status.ActiveBindings = []string{binding.Name}
			err = promotionRunReconciler.Create(ctx, promotionRun)
			Expect(err).To(BeNil())

			By("Trigger Reconciler.")
			_, err = promotionRunReconciler.Reconcile(ctx, request)
			Expect(err).To(BeNil())

			err = promotionRunReconciler.Client.Get(ctx, client.ObjectKeyFromObject(promotionRun), promotionRun)
			Expect(err).To(BeNil())
			Expect(len(promotionRun.Status.EnvironmentStatus) > 0).To(BeTrue())
			Expect(promotionRun.Status.EnvironmentStatus[0].Step).To(Equal(1))
			Expect(promotionRun.Status.EnvironmentStatus[0].DisplayStatus).To(Equal("Waiting for following GitOpsDeployments to be Synced/Healthy: " + gitOpsDeployment.Name))
			Expect(promotionRun.Status.EnvironmentStatus[0].Status).To(Equal(appstudiosharedv1.PromotionRunEnvironmentStatus_InProgress))
			Expect(promotionRun.Status.EnvironmentStatus[0].EnvironmentName).To(Equal(environment.Name))
		})

		It("Should create GitOpsDeployments and it should be Synced/Healthy.", func() {
			By("Create Snapshot CR.")
			snapshot := appstudiosharedv1.Snapshot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      promotionRun.Spec.Snapshot,
					Namespace: promotionRun.Namespace,
				},
				Spec: appstudiosharedv1.SnapshotSpec{
					Application: promotionRun.Spec.Application,
					DisplayName: promotionRun.Spec.Application,
				},
			}

			err := promotionRunReconciler.Create(ctx, &snapshot)
			Expect(err).To(BeNil())

			By("Create SnapshotEnvironmentBinding CR.")
			binding := &appstudiosharedv1.SnapshotEnvironmentBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "appa-staging-binding",
					Namespace: promotionRun.Namespace,
				},
				Spec: appstudiosharedv1.SnapshotEnvironmentBindingSpec{
					Application: promotionRun.Spec.Application,
					Environment: promotionRun.Spec.ManualPromotion.TargetEnvironment,
					Snapshot:    promotionRun.Spec.Snapshot,
					Components: []appstudiosharedv1.BindingComponent{
						{
							Name: "component-a",
							Configuration: appstudiosharedv1.BindingComponentConfiguration{
								Env: []appstudiosharedv1.EnvVarPair{
									{Name: "My_STG_ENV", Value: "1000"},
								},
								Replicas: 3,
							},
						},
					},
				},
				Status: appstudiosharedv1.SnapshotEnvironmentBindingStatus{
					GitOpsDeployments: []appstudiosharedv1.BindingStatusGitOpsDeployment{
						{
							ComponentName:    "component-a",
							GitOpsDeployment: "appa-staging-binding-" + promotionRun.Spec.Application + "-" + promotionRun.Spec.ManualPromotion.TargetEnvironment + "-component-a",
						},
					},
				},
			}

			err = promotionRunReconciler.Create(ctx, binding)
			Expect(err).To(BeNil())

			By("Create GitOpsDeployment CR.")
			gitOpsDeployment := &apibackend.GitOpsDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "appa-staging-binding-" + promotionRun.Spec.Application + "-" + promotionRun.Spec.ManualPromotion.TargetEnvironment + "-component-a",
					Namespace: binding.Namespace,
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: binding.APIVersion,
							Kind:       binding.Kind,
							Name:       binding.Name,
							UID:        binding.UID,
						},
					},
				},
				Spec: apibackend.GitOpsDeploymentSpec{
					Source: apibackend.ApplicationSource{
						RepoURL:        "https://github.com/redhat-appstudio/managed-gitops",
						Path:           "resources/test-data/sample-gitops-repository/components/componentA/overlays/staging",
						TargetRevision: "main",
					},
					Type:        apibackend.GitOpsDeploymentSpecType_Automated, // Default to automated, for now
					Destination: apibackend.ApplicationDestination{},           // Default to same namespace, for now
				},
			}

			err = promotionRunReconciler.Create(ctx, gitOpsDeployment)
			Expect(err).To(BeNil())

			By("Set the Active Bindings for Promotion.")
			promotionRun.Status.ActiveBindings = []string{binding.Name}
			err = promotionRunReconciler.Create(ctx, promotionRun)
			Expect(err).To(BeNil())

			By("Trigger Reconciler.")
			_, err = promotionRunReconciler.Reconcile(ctx, request)
			Expect(err).To(BeNil())

			err = promotionRunReconciler.Client.Get(ctx, client.ObjectKeyFromObject(promotionRun), promotionRun)
			Expect(err).To(BeNil())
			Expect(len(promotionRun.Status.EnvironmentStatus) > 0).To(BeTrue())
			Expect(promotionRun.Status.EnvironmentStatus[0].Step).To(Equal(1))
			Expect(promotionRun.Status.EnvironmentStatus[0].DisplayStatus).To(Equal(StatusMessageAllGitOpsDeploymentsAreSyncedHealthy))
			Expect(promotionRun.Status.EnvironmentStatus[0].Status).To(Equal(appstudiosharedv1.PromotionRunEnvironmentStatus_Success))
			Expect(promotionRun.Status.EnvironmentStatus[0].EnvironmentName).To(Equal(environment.Name))
		})

		It("Should fail if GitOpsDeployments are not Synced/Healthy in given time limit.", func() {
			By("Create Snapshot CR.")
			snapshot := appstudiosharedv1.Snapshot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      promotionRun.Spec.Snapshot,
					Namespace: promotionRun.Namespace,
				},
				Spec: appstudiosharedv1.SnapshotSpec{
					Application: promotionRun.Spec.Application,
					DisplayName: promotionRun.Spec.Application,
				},
			}

			err := promotionRunReconciler.Create(ctx, &snapshot)
			Expect(err).To(BeNil())

			By("Create SnapshotEnvironmentBinding CR.")
			binding := &appstudiosharedv1.SnapshotEnvironmentBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "appa-staging-binding",
					Namespace: promotionRun.Namespace,
				},
				Spec: appstudiosharedv1.SnapshotEnvironmentBindingSpec{
					Application: promotionRun.Spec.Application,
					Environment: promotionRun.Spec.ManualPromotion.TargetEnvironment,
					Snapshot:    promotionRun.Spec.Snapshot,
					Components: []appstudiosharedv1.BindingComponent{
						{
							Name: "component-a",
							Configuration: appstudiosharedv1.BindingComponentConfiguration{
								Env: []appstudiosharedv1.EnvVarPair{
									{Name: "My_STG_ENV", Value: "1000"},
								},
								Replicas: 3,
							},
						},
					},
				},
				Status: appstudiosharedv1.SnapshotEnvironmentBindingStatus{
					GitOpsDeployments: []appstudiosharedv1.BindingStatusGitOpsDeployment{
						{
							ComponentName:    "component-a",
							GitOpsDeployment: "appa-staging-binding-" + promotionRun.Spec.Application + "-" + promotionRun.Spec.ManualPromotion.TargetEnvironment + "-component-a",
						},
					},
				},
			}

			err = promotionRunReconciler.Create(ctx, binding)
			Expect(err).To(BeNil())

			By("Create GitOpsDeployment CR.")
			gitOpsDeployment := &apibackend.GitOpsDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "appa-staging-binding-" + promotionRun.Spec.Application + "-" + promotionRun.Spec.ManualPromotion.TargetEnvironment + "-component-a",
					Namespace: binding.Namespace,
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: binding.APIVersion,
							Kind:       binding.Kind,
							Name:       binding.Name,
							UID:        binding.UID,
						},
					},
				},
				Spec: apibackend.GitOpsDeploymentSpec{
					Source: apibackend.ApplicationSource{
						RepoURL:        "https://github.com/redhat-appstudio/managed-gitops",
						Path:           "resources/test-data/sample-gitops-repository/components/componentA/overlays/staging",
						TargetRevision: "main",
					},
					Type:        apibackend.GitOpsDeploymentSpecType_Automated, // Default to automated, for now
					Destination: apibackend.ApplicationDestination{},           // Default to same namespace, for now
				},
			}

			err = promotionRunReconciler.Create(ctx, gitOpsDeployment)
			Expect(err).To(BeNil())

			By("Set the Active Bindings for Promotion.")
			promotionRun.Status.ActiveBindings = []string{binding.Name}

			By("Set PromotionStartTime to 12Min before Now.")
			oldTime := metav1.Now().Add(time.Duration(-(PromotionRunTimeOutLimit + 2)) * time.Minute)
			promotionRun.Status.PromotionStartTime = metav1.NewTime(oldTime)

			err = promotionRunReconciler.Create(ctx, promotionRun)
			Expect(err).To(BeNil())

			By("Trigger Reconciler.")
			_, err = promotionRunReconciler.Reconcile(ctx, request)
			Expect(err).To(BeNil())

			checkStatusCondition(ctx, promotionRunReconciler.Client, promotionRun, fmt.Sprintf("Promotion Failed. Could not be completed in %d Minutes.", PromotionRunTimeOutLimit))
		})
	})
})

func checkStatusCondition(ctx context.Context, rClient client.Client, promotionRun *appstudiosharedv1.PromotionRun, message string) {
	err := rClient.Get(ctx, client.ObjectKeyFromObject(promotionRun), promotionRun)
	Expect(err).To(BeNil())
	Expect(len(promotionRun.Status.Conditions) > 0)

	for _, condition := range promotionRun.Status.Conditions {
		if condition.Type == appstudiosharedv1.PromotionRunConditionErrorOccurred {
			Expect(condition.Type).To(Equal(appstudiosharedv1.PromotionRunConditionErrorOccurred))
			Expect(condition.Message).To(Equal(message))
			Expect(condition.Status).To(Equal(appstudiosharedv1.PromotionRunConditionStatusTrue))
			Expect(condition.Reason).To(Equal(appstudiosharedv1.PromotionRunReasonErrorOccurred))
		}
	}
}
