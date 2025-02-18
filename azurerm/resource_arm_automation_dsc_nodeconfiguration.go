package azurerm

import (
	"fmt"
	"log"
	"strings"

	"github.com/Azure/azure-sdk-for-go/services/automation/mgmt/2015-10-31/automation"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/terraform-providers/terraform-provider-azurerm/azurerm/helpers/azure"
	"github.com/terraform-providers/terraform-provider-azurerm/azurerm/helpers/tf"
	"github.com/terraform-providers/terraform-provider-azurerm/azurerm/helpers/validate"
	"github.com/terraform-providers/terraform-provider-azurerm/azurerm/internal/features"
	"github.com/terraform-providers/terraform-provider-azurerm/azurerm/internal/timeouts"
	"github.com/terraform-providers/terraform-provider-azurerm/azurerm/utils"
)

func resourceArmAutomationDscNodeConfiguration() *schema.Resource {
	return &schema.Resource{
		Create: resourceArmAutomationDscNodeConfigurationCreateUpdate,
		Read:   resourceArmAutomationDscNodeConfigurationRead,
		Update: resourceArmAutomationDscNodeConfigurationCreateUpdate,
		Delete: resourceArmAutomationDscNodeConfigurationDelete,

		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		Schema: map[string]*schema.Schema{
			"name": {
				Type:         schema.TypeString,
				Required:     true,
				ForceNew:     true,
				ValidateFunc: validate.NoEmptyStrings,
			},

			"automation_account_name": {
				Type:         schema.TypeString,
				Required:     true,
				ForceNew:     true,
				ValidateFunc: validate.NoEmptyStrings,
			},

			"resource_group_name": azure.SchemaResourceGroupName(),

			"content_embedded": {
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: validate.NoEmptyStrings,
			},

			"configuration_name": {
				Type:     schema.TypeString,
				Computed: true,
			},
		},
	}
}

func resourceArmAutomationDscNodeConfigurationCreateUpdate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*ArmClient).Automation.DscNodeConfigurationClient
	ctx, cancel := timeouts.ForCreateUpdate(meta.(*ArmClient).StopContext, d)
	defer cancel()

	log.Printf("[INFO] preparing arguments for AzureRM Automation Dsc Node Configuration creation.")

	name := d.Get("name").(string)
	resGroup := d.Get("resource_group_name").(string)
	accName := d.Get("automation_account_name").(string)

	if features.ShouldResourcesBeImported() && d.IsNewResource() {
		existing, err := client.Get(ctx, resGroup, accName, name)
		if err != nil {
			if !utils.ResponseWasNotFound(existing.Response) {
				return fmt.Errorf("Error checking for presence of existing Automation DSC Node Configuration %q (Account %q / Resource Group %q): %s", name, accName, resGroup, err)
			}
		}

		if existing.ID != nil && *existing.ID != "" {
			return tf.ImportAsExistsError("azurerm_automation_dsc_nodeconfiguration", *existing.ID)
		}
	}

	content := d.Get("content_embedded").(string)

	// configuration name is always the first part of the dsc node configuration
	// e.g. webserver.prod or webserver.local will be associated to the dsc configuration webserver

	configurationName := strings.Split(name, ".")[0]

	parameters := automation.DscNodeConfigurationCreateOrUpdateParameters{
		Source: &automation.ContentSource{
			Type:  automation.EmbeddedContent,
			Value: utils.String(content),
		},
		Configuration: &automation.DscConfigurationAssociationProperty{
			Name: utils.String(configurationName),
		},
		Name: utils.String(name),
	}

	if _, err := client.CreateOrUpdate(ctx, resGroup, accName, name, parameters); err != nil {
		return err
	}

	read, err := client.Get(ctx, resGroup, accName, name)
	if err != nil {
		return err
	}

	if read.ID == nil {
		return fmt.Errorf("Cannot read Automation Dsc Node Configuration %q (resource group %q) ID", name, resGroup)
	}

	d.SetId(*read.ID)

	return resourceArmAutomationDscNodeConfigurationRead(d, meta)
}

func resourceArmAutomationDscNodeConfigurationRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*ArmClient).Automation.DscNodeConfigurationClient
	ctx, cancel := timeouts.ForRead(meta.(*ArmClient).StopContext, d)
	defer cancel()

	id, err := azure.ParseAzureResourceID(d.Id())
	if err != nil {
		return err
	}
	resGroup := id.ResourceGroup
	accName := id.Path["automationAccounts"]
	name := id.Path["nodeConfigurations"]

	resp, err := client.Get(ctx, resGroup, accName, name)
	if err != nil {
		if utils.ResponseWasNotFound(resp.Response) {
			d.SetId("")
			return nil
		}

		return fmt.Errorf("Error making Read request on AzureRM Automation Dsc Node Configuration %q: %+v", name, err)
	}

	d.Set("name", resp.Name)
	d.Set("resource_group_name", resGroup)
	d.Set("automation_account_name", accName)
	d.Set("configuration_name", resp.Configuration.Name)

	// cannot read back content_embedded as not part of body nor exposed through method

	return nil
}

func resourceArmAutomationDscNodeConfigurationDelete(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*ArmClient).Automation.DscNodeConfigurationClient
	ctx, cancel := timeouts.ForDelete(meta.(*ArmClient).StopContext, d)
	defer cancel()

	id, err := azure.ParseAzureResourceID(d.Id())
	if err != nil {
		return err
	}
	resGroup := id.ResourceGroup
	accName := id.Path["automationAccounts"]
	name := id.Path["nodeConfigurations"]

	resp, err := client.Delete(ctx, resGroup, accName, name)
	if err != nil {
		if utils.ResponseWasNotFound(resp) {
			return nil
		}

		return fmt.Errorf("Error issuing AzureRM delete request for Automation Dsc Node Configuration %q: %+v", name, err)
	}

	return nil
}
