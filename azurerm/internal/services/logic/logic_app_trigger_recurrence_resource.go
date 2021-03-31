package logic

import (
	"fmt"
	"log"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/helper/validation"
	"github.com/terraform-providers/terraform-provider-azurerm/azurerm/helpers/azure"
)

func resourceLogicAppTriggerRecurrence() *schema.Resource {
	return &schema.Resource{
		Create: resourceLogicAppTriggerRecurrenceCreateUpdate,
		Read:   resourceLogicAppTriggerRecurrenceRead,
		Update: resourceLogicAppTriggerRecurrenceCreateUpdate,
		Delete: resourceLogicAppTriggerRecurrenceDelete,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(30 * time.Minute),
			Read:   schema.DefaultTimeout(5 * time.Minute),
			Update: schema.DefaultTimeout(30 * time.Minute),
			Delete: schema.DefaultTimeout(30 * time.Minute),
		},

		Schema: map[string]*schema.Schema{
			"name": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},

			"logic_app_id": {
				Type:         schema.TypeString,
				Required:     true,
				ForceNew:     true,
				ValidateFunc: azure.ValidateResourceID,
			},

			"frequency": {
				Type:     schema.TypeString,
				Required: true,
				ValidateFunc: validation.StringInSlice([]string{
					"Month",
					"Week",
					"Day",
					"Hour",
					"Minute",
					"Hour",
					"Second",
				}, false),
			},

			"interval": {
				Type:     schema.TypeInt,
				Required: true,
			},

			"start_time": {
				Type:         schema.TypeString,
				Optional:     true,
				ValidateFunc: validation.IsRFC3339Time,
			},

			"schedule": {
				Type:     schema.TypeList,
				Optional: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"at_these_hours": {
							Type:         schema.TypeSet,
							Optional:     true,
							AtLeastOneOf: []string{"schedule.0.at_these_hours", "schedule.0.at_these_minutes", "schedule.0.on_these_days"},
							Elem: &schema.Schema{
								Type:         schema.TypeInt,
								ValidateFunc: validation.IntBetween(0, 23),
							},
						},
						"at_these_minutes": {
							Type:         schema.TypeSet,
							Optional:     true,
							AtLeastOneOf: []string{"schedule.0.at_these_hours", "schedule.0.at_these_minutes", "schedule.0.on_these_days"},
							Elem: &schema.Schema{
								Type:         schema.TypeInt,
								ValidateFunc: validation.IntBetween(0, 59),
							},
						},
						"on_these_days": {
							Type:         schema.TypeSet,
							Optional:     true,
							AtLeastOneOf: []string{"schedule.0.at_these_hours", "schedule.0.at_these_minutes", "schedule.0.on_these_days"},
							Elem: &schema.Schema{
								Type: schema.TypeString,
								ValidateFunc: validation.StringInSlice([]string{
									"Monday",
									"Tuesday",
									"Wednesday",
									"Thursday",
									"Friday",
									"Saturday",
									"Sunday",
								}, false),
							},
						},
					},
				},
			},

			"time_zone": {
				Type:         schema.TypeString,
				Optional:     true,
				Computed:     true,
				ValidateFunc: validateLogicAppTriggerRecurrenceTimeZone(),
			},
		},
	}
}

func resourceLogicAppTriggerRecurrenceCreateUpdate(d *schema.ResourceData, meta interface{}) error {
	trigger := map[string]interface{}{
		"recurrence": map[string]interface{}{
			"frequency": d.Get("frequency").(string),
			"interval":  d.Get("interval").(int),
		},
		"type": "Recurrence",
	}

	if v, ok := d.GetOk("start_time"); ok {
		trigger["recurrence"].(map[string]interface{})["startTime"] = v.(string)

		// time_zone only allowed when start_time is specified
		if v, ok := d.GetOk("time_zone"); ok {
			trigger["recurrence"].(map[string]interface{})["timeZone"] = v.(string)
		}
	}

	if v, ok := d.GetOk("schedule"); ok {
		trigger["recurrence"].(map[string]interface{})["schedule"] = expandLogicAppTriggerRecurrenceSchedule(v.([]interface{}))
	}

	logicAppId := d.Get("logic_app_id").(string)
	name := d.Get("name").(string)
	if err := resourceLogicAppTriggerUpdate(d, meta, logicAppId, name, trigger, "azurerm_logic_app_trigger_recurrence"); err != nil {
		return err
	}

	return resourceLogicAppTriggerRecurrenceRead(d, meta)
}

func resourceLogicAppTriggerRecurrenceRead(d *schema.ResourceData, meta interface{}) error {
	id, err := azure.ParseAzureResourceID(d.Id())
	if err != nil {
		return err
	}

	resourceGroup := id.ResourceGroup
	logicAppName := id.Path["workflows"]
	name := id.Path["triggers"]

	t, app, err := retrieveLogicAppTrigger(d, meta, resourceGroup, logicAppName, name)
	if err != nil {
		return err
	}

	if t == nil {
		log.Printf("[DEBUG] Logic App %q (Resource Group %q) does not contain Trigger %q - removing from state", logicAppName, resourceGroup, name)
		d.SetId("")
		return nil
	}

	trigger := *t

	d.Set("name", name)
	d.Set("logic_app_id", app.ID)

	v := trigger["recurrence"]
	if v == nil {
		return fmt.Errorf("Error `recurrence` was nil for HTTP Trigger %q (Logic App %q / Resource Group %q)", name, logicAppName, resourceGroup)
	}

	recurrence, ok := v.(map[string]interface{})
	if !ok {
		return fmt.Errorf("Error parsing `recurrence` for HTTP Trigger %q (Logic App %q / Resource Group %q)", name, logicAppName, resourceGroup)
	}

	if frequency := recurrence["frequency"]; frequency != nil {
		d.Set("frequency", frequency.(string))
	}

	if interval := recurrence["interval"]; interval != nil {
		d.Set("interval", int(interval.(float64)))
	}

	if startTime := recurrence["startTime"]; startTime != nil {
		d.Set("start_time", startTime.(string))
	}

	if timeZone := recurrence["timeZone"]; timeZone != nil {
		d.Set("time_zone", timeZone.(string))
	}

	if schedule := recurrence["schedule"]; schedule != nil {
		d.Set("schedule", flattenLogicAppTriggerRecurrenceSchedule(schedule.(map[string]interface{})))
	}

	return nil
}

func resourceLogicAppTriggerRecurrenceDelete(d *schema.ResourceData, meta interface{}) error {
	id, err := azure.ParseAzureResourceID(d.Id())
	if err != nil {
		return err
	}

	resourceGroup := id.ResourceGroup
	logicAppName := id.Path["workflows"]
	name := id.Path["triggers"]

	err = resourceLogicAppTriggerRemove(d, meta, resourceGroup, logicAppName, name)
	if err != nil {
		return fmt.Errorf("Error removing Trigger %q from Logic App %q (Resource Group %q): %+v", name, logicAppName, resourceGroup, err)
	}

	return nil
}

func validateLogicAppTriggerRecurrenceTimeZone() schema.SchemaValidateFunc {
	// from https://support.microsoft.com/en-us/help/973627/microsoft-time-zone-index-values
	timeZones := []string{
		"Dateline Standard Time",
		"Samoa Standard Time",
		"Hawaiian Standard Time",
		"Alaskan Standard Time",
		"Pacific Standard Time",
		"Mountain Standard Time",
		"Mexico Standard Time",
		"US Mountain Standard Time",
		"Central Standard Time",
		"Canada Central Standard Time",
		"Mexico Standard Time",
		"Central America Standard Time",
		"Eastern Standard Time",
		"US Eastern Standard Time",
		"SA Pacific Standard Time",
		"Atlantic Standard Time",
		"SA Western Standard Time",
		"Pacific SA Standard Time",
		"Newfoundland and Labrador Standard Time",
		"E South America Standard Time",
		"SA Eastern Standard Time",
		"Greenland Standard Time",
		"Mid-Atlantic Standard Time",
		"Azores Standard Time",
		"Cape Verde Standard Time",
		"GMT Standard Time",
		"Greenwich Standard Time",
		"Central Europe Standard Time",
		"Central European Standard Time",
		"Romance Standard Time",
		"W Europe Standard Time",
		"W Central Africa Standard Time",
		"E Europe Standard Time",
		"Egypt Standard Time",
		"FLE Standard Time",
		"GTB Standard Time",
		"Israel Standard Time",
		"South Africa Standard Time",
		"Russian Standard Time",
		"Arab Standard Time",
		"E Africa Standard Time",
		"Arabic Standard Time",
		"Iran Standard Time",
		"Arabian Standard Time",
		"Caucasus Standard Time",
		"Transitional Islamic State of Afghanistan Standard Time",
		"Ekaterinburg Standard Time",
		"West Asia Standard Time",
		"India Standard Time",
		"Nepal Standard Time",
		"Central Asia Standard Time",
		"Sri Lanka Standard Time",
		"N Central Asia Standard Time",
		"Myanmar Standard Time",
		"SE Asia Standard Time",
		"North Asia Standard Time",
		"China Standard Time",
		"Singapore Standard Time",
		"Taipei Standard Time",
		"W Australia Standard Time",
		"North Asia East Standard Time",
		"Korea Standard Time",
		"Tokyo Standard Time",
		"Yakutsk Standard Time",
		"AUS Central Standard Time",
		"Cen Australia Standard Time",
		"AUS Eastern Standard Time",
		"E Australia Standard Time",
		"Tasmania Standard Time",
		"Vladivostok Standard Time",
		"West Pacific Standard Time",
		"Central Pacific Standard Time",
		"Fiji Islands Standard Time",
		"New Zealand Standard Time",
		"Tonga Standard Time",
		"Azerbaijan Standard Time",
		"Middle East Standard Time",
		"Jordan Standard Time",
		"Central Standard Time (Mexico)",
		"Mountain Standard Time (Mexico)",
		"Pacific Standard Time (Mexico)",
		"Namibia Standard Time",
		"Georgian Standard Time",
		"Central Brazilian Standard Time",
		"Montevideo Standard Time",
		"Armenian Standard Time",
		"Venezuela Standard Time",
		"Argentina Standard Time",
		"Morocco Standard Time",
		"Pakistan Standard Time",
		"Mauritius Standard Time",
		"UTC",
		"Paraguay Standard Time",
		"Kamchatka Standard Time",
	}
	return validation.StringInSlice(timeZones, false)
}

func expandLogicAppTriggerRecurrenceSchedule(input []interface{}) map[string]interface{} {
	output := make(map[string]interface{})
	if len(input) == 0 || input[0] == nil {
		return output
	}

	attrs := input[0].(map[string]interface{})
	if hoursRaw, ok := attrs["at_these_hours"]; ok {
		hoursSet := hoursRaw.(*schema.Set).List()
		hours := make([]int, 0)
		for _, hour := range hoursSet {
			hours = append(hours, hour.(int))
		}
		if len(hours) > 0 {
			output["hours"] = &hours
		}
	}
	if minutesRaw, ok := attrs["at_these_minutes"]; ok {
		minutesSet := minutesRaw.(*schema.Set).List()
		minutes := make([]int, 0)
		for _, minute := range minutesSet {
			minutes = append(minutes, minute.(int))
		}
		if len(minutes) > 0 {
			output["minutes"] = &minutes
		}
	}
	if daysRaw, ok := attrs["on_these_days"]; ok {
		daysSet := daysRaw.(*schema.Set).List()
		days := make([]string, 0)
		for _, day := range daysSet {
			days = append(days, day.(string))
		}
		if len(days) > 0 {
			output["weekDays"] = &days
		}
	}

	return output
}

func flattenLogicAppTriggerRecurrenceSchedule(input map[string]interface{}) []interface{} {
	attrs := make(map[string]interface{})

	if hours := input["hours"]; hours != nil {
		attrs["at_these_hours"] = hours
	}
	if minutes := input["minutes"]; minutes != nil {
		attrs["at_these_minutes"] = minutes
	}
	if days := input["weekDays"]; days != nil {
		attrs["on_these_days"] = days
	}

	return []interface{}{attrs}
}
