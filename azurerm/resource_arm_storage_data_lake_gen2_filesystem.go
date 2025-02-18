package azurerm

import (
	"fmt"
	"log"
	"regexp"

	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/terraform-providers/terraform-provider-azurerm/azurerm/helpers/tf"
	"github.com/terraform-providers/terraform-provider-azurerm/azurerm/internal/features"
	"github.com/terraform-providers/terraform-provider-azurerm/azurerm/internal/services/storage"
	"github.com/terraform-providers/terraform-provider-azurerm/azurerm/internal/timeouts"
	"github.com/terraform-providers/terraform-provider-azurerm/azurerm/utils"
	"github.com/tombuildsstuff/giovanni/storage/2018-11-09/datalakestore/filesystems"
)

func resourceArmStorageDataLakeGen2FileSystem() *schema.Resource {
	return &schema.Resource{
		Create: resourceArmStorageDataLakeGen2FileSystemCreate,
		Read:   resourceArmStorageDataLakeGen2FileSystemRead,
		Update: resourceArmStorageDataLakeGen2FileSystemUpdate,
		Delete: resourceArmStorageDataLakeGen2FileSystemDelete,

		Importer: &schema.ResourceImporter{
			State: func(d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
				storageClients := meta.(*ArmClient).Storage
				ctx := meta.(*ArmClient).StopContext

				id, err := filesystems.ParseResourceID(d.Id())
				if err != nil {
					return []*schema.ResourceData{d}, fmt.Errorf("Error parsing ID %q for import of Data Lake Gen2 File System: %v", d.Id(), err)
				}

				// we then need to look up the Storage Account ID - so first find the resource group
				resourceGroup, err := storageClients.FindResourceGroup(ctx, id.AccountName)
				if err != nil {
					return []*schema.ResourceData{d}, fmt.Errorf("Error locating Resource Group for Storage Account %q to import Data Lake Gen2 File System %q: %v", id.AccountName, d.Id(), err)
				}

				if resourceGroup == nil {
					return []*schema.ResourceData{d}, fmt.Errorf("Unable to locate Resource Group for Storage Account %q to import Data Lake Gen2 File System %q", id.AccountName, d.Id())
				}

				// then pull the storage account itself
				account, err := storageClients.AccountsClient.GetProperties(ctx, *resourceGroup, id.AccountName, "")
				if err != nil {
					return []*schema.ResourceData{d}, fmt.Errorf("Error retrieving Storage Account %q to import Data Lake Gen2 File System %q: %+v", id.AccountName, d.Id(), err)
				}

				d.Set("storage_account_id", account.ID)

				return []*schema.ResourceData{d}, nil
			},
		},

		Schema: map[string]*schema.Schema{
			"name": {
				Type:         schema.TypeString,
				Required:     true,
				ForceNew:     true,
				ValidateFunc: validateArmStorageDataLakeGen2FileSystemName,
			},

			"storage_account_id": storage.AccountIDSchema(),

			"properties": storage.MetaDataSchema(),
		},
	}
}

func resourceArmStorageDataLakeGen2FileSystemCreate(d *schema.ResourceData, meta interface{}) error {
	accountsClient := meta.(*ArmClient).Storage.AccountsClient
	client := meta.(*ArmClient).Storage.FileSystemsClient
	ctx, cancel := timeouts.ForCreate(meta.(*ArmClient).StopContext, d)
	defer cancel()

	storageID, err := storage.ParseAccountID(d.Get("storage_account_id").(string))
	if err != nil {
		return err
	}

	// confirm the storage account exists, otherwise Data Plane API requests will fail
	storageAccount, err := accountsClient.GetProperties(ctx, storageID.ResourceGroup, storageID.Name, "")
	if err != nil {
		if utils.ResponseWasNotFound(storageAccount.Response) {
			return fmt.Errorf("Storage Account %q was not found in Resource Group %q!", storageID.Name, storageID.ResourceGroup)
		}

		return fmt.Errorf("Error checking for existence of Storage Account %q (Resource Group %q): %+v", storageID.Name, storageID.ResourceGroup, err)
	}

	fileSystemName := d.Get("name").(string)
	propertiesRaw := d.Get("properties").(map[string]interface{})
	properties := storage.ExpandMetaData(propertiesRaw)

	id := client.GetResourceID(storageID.Name, fileSystemName)

	if features.ShouldResourcesBeImported() {
		resp, err := client.GetProperties(ctx, storageID.Name, fileSystemName)
		if err != nil {
			if !utils.ResponseWasNotFound(resp.Response) {
				return fmt.Errorf("Error checking for existence of existing File System %q (Account %q): %+v", fileSystemName, storageID.Name, err)
			}
		}

		if !utils.ResponseWasNotFound(resp.Response) {
			return tf.ImportAsExistsError("azurerm_storage_data_lake_gen2_filesystem", id)
		}
	}

	log.Printf("[INFO] Creating File System %q in Storage Account %q.", fileSystemName, storageID.Name)
	input := filesystems.CreateInput{
		Properties: properties,
	}
	if _, err := client.Create(ctx, storageID.Name, fileSystemName, input); err != nil {
		return fmt.Errorf("Error creating File System %q in Storage Account %q: %s", fileSystemName, storageID.Name, err)
	}

	d.SetId(id)
	return resourceArmStorageDataLakeGen2FileSystemRead(d, meta)
}

func resourceArmStorageDataLakeGen2FileSystemUpdate(d *schema.ResourceData, meta interface{}) error {
	accountsClient := meta.(*ArmClient).Storage.AccountsClient
	client := meta.(*ArmClient).Storage.FileSystemsClient
	ctx, cancel := timeouts.ForUpdate(meta.(*ArmClient).StopContext, d)
	defer cancel()

	id, err := filesystems.ParseResourceID(d.Id())
	if err != nil {
		return err
	}

	storageID, err := storage.ParseAccountID(d.Get("storage_account_id").(string))
	if err != nil {
		return err
	}

	// confirm the storage account exists, otherwise Data Plane API requests will fail
	storageAccount, err := accountsClient.GetProperties(ctx, storageID.ResourceGroup, storageID.Name, "")
	if err != nil {
		if utils.ResponseWasNotFound(storageAccount.Response) {
			return fmt.Errorf("Storage Account %q was not found in Resource Group %q!", storageID.Name, storageID.ResourceGroup)
		}

		return fmt.Errorf("Error checking for existence of Storage Account %q (Resource Group %q): %+v", storageID.Name, storageID.ResourceGroup, err)
	}

	propertiesRaw := d.Get("properties").(map[string]interface{})
	properties := storage.ExpandMetaData(propertiesRaw)

	log.Printf("[INFO] Updating Properties for File System %q in Storage Account %q.", id.DirectoryName, id.AccountName)
	input := filesystems.SetPropertiesInput{
		Properties: properties,
	}
	if _, err = client.SetProperties(ctx, id.AccountName, id.DirectoryName, input); err != nil {
		return fmt.Errorf("Error updating Properties for File System %q in Storage Account %q: %s", id.DirectoryName, id.AccountName, err)
	}

	return resourceArmStorageDataLakeGen2FileSystemRead(d, meta)
}

func resourceArmStorageDataLakeGen2FileSystemRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*ArmClient).Storage.FileSystemsClient
	ctx, cancel := timeouts.ForRead(meta.(*ArmClient).StopContext, d)
	defer cancel()

	id, err := filesystems.ParseResourceID(d.Id())
	if err != nil {
		return err
	}

	// TODO: what about when this has been removed?
	resp, err := client.GetProperties(ctx, id.AccountName, id.DirectoryName)
	if err != nil {
		if utils.ResponseWasNotFound(resp.Response) {
			log.Printf("[INFO] File System %q does not exist in Storage Account %q - removing from state...", id.DirectoryName, id.AccountName)
			d.SetId("")
			return nil
		}

		return fmt.Errorf("Error retrieving File System %q in Storage Account %q: %+v", id.DirectoryName, id.AccountName, err)
	}

	d.Set("name", id.DirectoryName)

	if err := d.Set("properties", resp.Properties); err != nil {
		return fmt.Errorf("Error setting `properties`: %+v", err)
	}

	return nil
}

func resourceArmStorageDataLakeGen2FileSystemDelete(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*ArmClient).Storage.FileSystemsClient
	ctx, cancel := timeouts.ForDelete(meta.(*ArmClient).StopContext, d)
	defer cancel()

	id, err := filesystems.ParseResourceID(d.Id())
	if err != nil {
		return err
	}

	resp, err := client.Delete(ctx, id.AccountName, id.DirectoryName)
	if err != nil {
		if !utils.ResponseWasNotFound(resp) {
			return fmt.Errorf("Error deleting File System %q in Storage Account %q: %+v", id.DirectoryName, id.AccountName, err)
		}
	}

	return nil
}

func validateArmStorageDataLakeGen2FileSystemName(v interface{}, k string) (warnings []string, errors []error) {
	value := v.(string)
	if !regexp.MustCompile(`^\$root$|^[0-9a-z-]+$`).MatchString(value) {
		errors = append(errors, fmt.Errorf(
			"only lowercase alphanumeric characters and hyphens allowed in %q: %q",
			k, value))
	}
	if len(value) < 3 || len(value) > 63 {
		errors = append(errors, fmt.Errorf(
			"%q must be between 3 and 63 characters: %q", k, value))
	}
	if regexp.MustCompile(`^-`).MatchString(value) {
		errors = append(errors, fmt.Errorf(
			"%q cannot begin with a hyphen: %q", k, value))
	}
	return warnings, errors
}
