package azurerm

import (
	"fmt"
	"log"

	"github.com/Azure/azure-sdk-for-go/services/web/mgmt/2018-02-01/web"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/helper/validation"
	"github.com/terraform-providers/terraform-provider-azurerm/azurerm/helpers/azure"
	"github.com/terraform-providers/terraform-provider-azurerm/azurerm/helpers/tf"
	"github.com/terraform-providers/terraform-provider-azurerm/azurerm/helpers/validate"
	"github.com/terraform-providers/terraform-provider-azurerm/azurerm/internal/features"
	"github.com/terraform-providers/terraform-provider-azurerm/azurerm/internal/locks"
	"github.com/terraform-providers/terraform-provider-azurerm/azurerm/internal/timeouts"
	"github.com/terraform-providers/terraform-provider-azurerm/azurerm/utils"
)

var appServiceCustomHostnameBindingResourceName = "azurerm_app_service_custom_hostname_binding"

func resourceArmAppServiceCustomHostnameBinding() *schema.Resource {
	return &schema.Resource{
		Create: resourceArmAppServiceCustomHostnameBindingCreate,
		Read:   resourceArmAppServiceCustomHostnameBindingRead,
		Delete: resourceArmAppServiceCustomHostnameBindingDelete,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		Schema: map[string]*schema.Schema{
			"hostname": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},

			"resource_group_name": azure.SchemaResourceGroupName(),

			"app_service_name": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},

			"ssl_state": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
				ValidateFunc: validation.StringInSlice([]string{
					string(web.SslStateIPBasedEnabled),
					string(web.SslStateSniEnabled),
				}, false),
			},

			"thumbprint": {
				Type:         schema.TypeString,
				Optional:     true,
				ForceNew:     true,
				ValidateFunc: validate.NoEmptyStrings,
			},

			"virtual_ip": {
				Type:     schema.TypeString,
				Computed: true,
			},
		},
	}
}

func resourceArmAppServiceCustomHostnameBindingCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*ArmClient).Web.AppServicesClient
	ctx, cancel := timeouts.ForCreate(meta.(*ArmClient).StopContext, d)
	defer cancel()

	log.Printf("[INFO] preparing arguments for App Service Hostname Binding creation.")

	resourceGroup := d.Get("resource_group_name").(string)
	appServiceName := d.Get("app_service_name").(string)
	hostname := d.Get("hostname").(string)
	sslState := d.Get("ssl_state").(string)
	thumbprint := d.Get("thumbprint").(string)

	locks.ByName(appServiceName, appServiceCustomHostnameBindingResourceName)
	defer locks.UnlockByName(appServiceName, appServiceCustomHostnameBindingResourceName)

	if features.ShouldResourcesBeImported() && d.IsNewResource() {
		existing, err := client.GetHostNameBinding(ctx, resourceGroup, appServiceName, hostname)
		if err != nil {
			if !utils.ResponseWasNotFound(existing.Response) {
				return fmt.Errorf("Error checking for presence of existing Custom Hostname Binding %q (App Service %q / Resource Group %q): %s", hostname, appServiceName, resourceGroup, err)
			}
		}

		if existing.ID != nil && *existing.ID != "" {
			return tf.ImportAsExistsError("azurerm_app_service_custom_hostname_binding", *existing.ID)
		}
	}

	properties := web.HostNameBinding{
		HostNameBindingProperties: &web.HostNameBindingProperties{
			SiteName: utils.String(appServiceName),
		},
	}

	if sslState != "" {
		if thumbprint == "" {
			return fmt.Errorf("`thumbprint` must be specified when `ssl_state` is set")
		}

		properties.HostNameBindingProperties.SslState = web.SslState(sslState)
	}

	if thumbprint != "" {
		if sslState == "" {
			return fmt.Errorf("`ssl_state` must be specified when `thumbprint` is set")
		}

		properties.HostNameBindingProperties.Thumbprint = utils.String(thumbprint)
	}

	if _, err := client.CreateOrUpdateHostNameBinding(ctx, resourceGroup, appServiceName, hostname, properties); err != nil {
		return err
	}

	read, err := client.GetHostNameBinding(ctx, resourceGroup, appServiceName, hostname)
	if err != nil {
		return err
	}
	if read.ID == nil {
		return fmt.Errorf("Cannot read Hostname Binding %q (App Service %q / Resource Group %q) ID", hostname, appServiceName, resourceGroup)
	}

	d.SetId(*read.ID)

	return resourceArmAppServiceCustomHostnameBindingRead(d, meta)
}

func resourceArmAppServiceCustomHostnameBindingRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*ArmClient).Web.AppServicesClient
	ctx, cancel := timeouts.ForRead(meta.(*ArmClient).StopContext, d)
	defer cancel()

	id, err := azure.ParseAzureResourceID(d.Id())
	if err != nil {
		return err
	}

	resourceGroup := id.ResourceGroup
	appServiceName := id.Path["sites"]
	hostname := id.Path["hostNameBindings"]

	resp, err := client.GetHostNameBinding(ctx, resourceGroup, appServiceName, hostname)
	if err != nil {
		if utils.ResponseWasNotFound(resp.Response) {
			log.Printf("[DEBUG] App Service Hostname Binding %q (App Service %q / Resource Group %q) was not found - removing from state", hostname, appServiceName, resourceGroup)
			d.SetId("")
			return nil
		}
		return fmt.Errorf("Error making Read request on App Service Hostname Binding %q (App Service %q / Resource Group %q): %+v", hostname, appServiceName, resourceGroup, err)
	}

	d.Set("hostname", hostname)
	d.Set("app_service_name", appServiceName)
	d.Set("resource_group_name", resourceGroup)

	if props := resp.HostNameBindingProperties; props != nil {
		d.Set("ssl_state", props.SslState)
		d.Set("thumbprint", props.Thumbprint)
		d.Set("virtual_ip", props.VirtualIP)
	}

	return nil
}

func resourceArmAppServiceCustomHostnameBindingDelete(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*ArmClient).Web.AppServicesClient
	ctx, cancel := timeouts.ForDelete(meta.(*ArmClient).StopContext, d)
	defer cancel()

	id, err := azure.ParseAzureResourceID(d.Id())
	if err != nil {
		return err
	}
	resGroup := id.ResourceGroup
	appServiceName := id.Path["sites"]
	hostname := id.Path["hostNameBindings"]

	locks.ByName(appServiceName, appServiceCustomHostnameBindingResourceName)
	defer locks.UnlockByName(appServiceName, appServiceCustomHostnameBindingResourceName)

	log.Printf("[DEBUG] Deleting App Service Hostname Binding %q (App Service %q / Resource Group %q)", hostname, appServiceName, resGroup)

	resp, err := client.DeleteHostNameBinding(ctx, resGroup, appServiceName, hostname)
	if err != nil {
		if !utils.ResponseWasNotFound(resp) {
			return err
		}
	}

	return nil
}
