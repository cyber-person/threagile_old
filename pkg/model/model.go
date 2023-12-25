/*
Copyright © 2023 NAME HERE <EMAIL ADDRESS>
*/
package model

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/threagile/threagile/pkg/input"
	"github.com/threagile/threagile/pkg/security/types"
)

type ParsedModel struct {
	Author                                        input.Author
	Title                                         string
	Date                                          time.Time
	ManagementSummaryComment                      string
	BusinessOverview                              input.Overview
	TechnicalOverview                             input.Overview
	BusinessCriticality                           types.Criticality
	SecurityRequirements                          map[string]string
	Questions                                     map[string]string
	AbuseCases                                    map[string]string
	TagsAvailable                                 []string
	DataAssets                                    map[string]DataAsset
	TechnicalAssets                               map[string]TechnicalAsset
	TrustBoundaries                               map[string]TrustBoundary
	SharedRuntimes                                map[string]SharedRuntime
	IndividualRiskCategories                      map[string]RiskCategory
	RiskTracking                                  map[string]RiskTracking
	CommunicationLinks                            map[string]CommunicationLink
	AllSupportedTags                              map[string]bool
	DiagramTweakNodesep, DiagramTweakRanksep      int
	DiagramTweakEdgeLayout                        string
	DiagramTweakSuppressEdgeLabels                bool
	DiagramTweakLayoutLeftToRight                 bool
	DiagramTweakInvisibleConnectionsBetweenAssets []string
	DiagramTweakSameRankAssets                    []string

	// TODO: those are generated based on items above and needs to be private
	IncomingTechnicalCommunicationLinksMappedByTargetId   map[string][]CommunicationLink
	DirectContainingTrustBoundaryMappedByTechnicalAssetId map[string]TrustBoundary
	GeneratedRisksByCategory                              map[RiskCategory][]Risk
	GeneratedRisksBySyntheticId                           map[string]Risk
}

func ParseModel(modelInput *input.ModelInput) (*ParsedModel, error) {
	businessCriticality, err := types.ParseCriticality(modelInput.BusinessCriticality)
	if err != nil {
		panic(errors.New("unknown 'business_criticality' value of application: " + modelInput.BusinessCriticality))
	}

	reportDate := time.Now()
	if len(modelInput.Date) > 0 {
		var parseError error
		reportDate, parseError = time.Parse("2006-01-02", modelInput.Date)
		if parseError != nil {
			panic(errors.New("unable to parse 'date' value of model file"))
		}
	}

	parsedModel := ParsedModel{
		Author:                         modelInput.Author,
		Title:                          modelInput.Title,
		Date:                           reportDate,
		ManagementSummaryComment:       modelInput.ManagementSummaryComment,
		BusinessCriticality:            businessCriticality,
		BusinessOverview:               removePathElementsFromImageFiles(modelInput.BusinessOverview),
		TechnicalOverview:              removePathElementsFromImageFiles(modelInput.TechnicalOverview),
		Questions:                      modelInput.Questions,
		AbuseCases:                     modelInput.AbuseCases,
		SecurityRequirements:           modelInput.SecurityRequirements,
		TagsAvailable:                  lowerCaseAndTrim(modelInput.TagsAvailable),
		DiagramTweakNodesep:            modelInput.DiagramTweakNodesep,
		DiagramTweakRanksep:            modelInput.DiagramTweakRanksep,
		DiagramTweakEdgeLayout:         modelInput.DiagramTweakEdgeLayout,
		DiagramTweakSuppressEdgeLabels: modelInput.DiagramTweakSuppressEdgeLabels,
		DiagramTweakLayoutLeftToRight:  modelInput.DiagramTweakLayoutLeftToRight,
		DiagramTweakInvisibleConnectionsBetweenAssets: modelInput.DiagramTweakInvisibleConnectionsBetweenAssets,
		DiagramTweakSameRankAssets:                    modelInput.DiagramTweakSameRankAssets,
	}

	parsedModel.CommunicationLinks = make(map[string]CommunicationLink)
	parsedModel.IncomingTechnicalCommunicationLinksMappedByTargetId = make(map[string][]CommunicationLink)
	parsedModel.DirectContainingTrustBoundaryMappedByTechnicalAssetId = make(map[string]TrustBoundary)
	parsedModel.GeneratedRisksByCategory = make(map[RiskCategory][]Risk)
	parsedModel.GeneratedRisksBySyntheticId = make(map[string]Risk)
	parsedModel.AllSupportedTags = make(map[string]bool)

	if parsedModel.DiagramTweakNodesep == 0 {
		parsedModel.DiagramTweakNodesep = 2
	}
	if parsedModel.DiagramTweakRanksep == 0 {
		parsedModel.DiagramTweakRanksep = 2
	}

	// Data Assets ===============================================================================
	parsedModel.DataAssets = make(map[string]DataAsset)
	for title, asset := range modelInput.DataAssets {
		id := fmt.Sprintf("%v", asset.ID)

		usage, err := types.ParseUsage(asset.Usage)
		if err != nil {
			panic(errors.New("unknown 'usage' value of data asset '" + title + "': " + asset.Usage))
		}
		quantity, err := types.ParseQuantity(asset.Quantity)
		if err != nil {
			panic(errors.New("unknown 'quantity' value of data asset '" + title + "': " + asset.Quantity))
		}
		confidentiality, err := types.ParseConfidentiality(asset.Confidentiality)
		if err != nil {
			panic(errors.New("unknown 'confidentiality' value of data asset '" + title + "': " + asset.Confidentiality))
		}
		integrity, err := types.ParseCriticality(asset.Integrity)
		if err != nil {
			panic(errors.New("unknown 'integrity' value of data asset '" + title + "': " + asset.Integrity))
		}
		availability, err := types.ParseCriticality(asset.Availability)
		if err != nil {
			panic(errors.New("unknown 'availability' value of data asset '" + title + "': " + asset.Availability))
		}

		checkIdSyntax(id)
		if _, exists := parsedModel.DataAssets[id]; exists {
			panic(errors.New("duplicate id used: " + id))
		}
		parsedModel.DataAssets[id] = DataAsset{
			Id:                     id,
			Title:                  title,
			Usage:                  usage,
			Description:            withDefault(fmt.Sprintf("%v", asset.Description), title),
			Quantity:               quantity,
			Tags:                   parsedModel.checkTags(lowerCaseAndTrim(asset.Tags), "data asset '"+title+"'"),
			Origin:                 fmt.Sprintf("%v", asset.Origin),
			Owner:                  fmt.Sprintf("%v", asset.Owner),
			Confidentiality:        confidentiality,
			Integrity:              integrity,
			Availability:           availability,
			JustificationCiaRating: fmt.Sprintf("%v", asset.JustificationCiaRating),
		}
	}

	// Technical Assets ===============================================================================
	parsedModel.TechnicalAssets = make(map[string]TechnicalAsset)
	for title, asset := range modelInput.TechnicalAssets {
		id := fmt.Sprintf("%v", asset.ID)

		usage, err := types.ParseUsage(asset.Usage)
		if err != nil {
			panic(errors.New("unknown 'usage' value of technical asset '" + title + "': " + asset.Usage))
		}

		var dataAssetsProcessed = make([]string, 0)
		if asset.DataAssetsProcessed != nil {
			dataAssetsProcessed = make([]string, len(asset.DataAssetsProcessed))
			for i, parsedProcessedAsset := range asset.DataAssetsProcessed {
				referencedAsset := fmt.Sprintf("%v", parsedProcessedAsset)
				err := parsedModel.checkDataAssetTargetExists(referencedAsset, "technical asset '"+title+"'")
				if err != nil {
					return nil, err
				}
				dataAssetsProcessed[i] = referencedAsset
			}
		}

		var dataAssetsStored = make([]string, 0)
		if asset.DataAssetsStored != nil {
			dataAssetsStored = make([]string, len(asset.DataAssetsStored))
			for i, parsedStoredAssets := range asset.DataAssetsStored {
				referencedAsset := fmt.Sprintf("%v", parsedStoredAssets)
				err := parsedModel.checkDataAssetTargetExists(referencedAsset, "technical asset '"+title+"'")
				if err != nil {
					return nil, err
				}
				dataAssetsStored[i] = referencedAsset
			}
		}

		technicalAssetType, err := types.ParseTechnicalAssetType(asset.Type)
		if err != nil {
			return nil, errors.New("unknown 'type' value of technical asset '" + title + "': " + fmt.Sprintf("%v", asset.Type))
		}
		technicalAssetSize, err := types.ParseTechnicalAssetSize(asset.Size)
		if err != nil {
			return nil, errors.New("unknown 'size' value of technical asset '" + title + "': " + fmt.Sprintf("%v", asset.Size))
		}
		technicalAssetTechnology, err := types.ParseTechnicalAssetTechnology(asset.Technology)
		if err != nil {
			return nil, errors.New("unknown 'technology' value of technical asset '" + title + "': " + fmt.Sprintf("%v", asset.Technology))
		}
		encryption, err := types.ParseEncryptionStyle(asset.Encryption)
		if err != nil {
			return nil, errors.New("unknown 'encryption' value of technical asset '" + title + "': " + fmt.Sprintf("%v", asset.Encryption))
		}
		technicalAssetMachine, err := types.ParseTechnicalAssetMachine(asset.Machine)
		if err != nil {
			return nil, errors.New("unknown 'machine' value of technical asset '" + title + "': " + fmt.Sprintf("%v", asset.Machine))
		}
		confidentiality, err := types.ParseConfidentiality(asset.Confidentiality)
		if err != nil {
			return nil, errors.New("unknown 'confidentiality' value of technical asset '" + title + "': " + fmt.Sprintf("%v", asset.Confidentiality))
		}
		integrity, err := types.ParseCriticality(asset.Integrity)
		if err != nil {
			return nil, errors.New("unknown 'integrity' value of technical asset '" + title + "': " + fmt.Sprintf("%v", asset.Integrity))
		}
		availability, err := types.ParseCriticality(asset.Availability)
		if err != nil {
			return nil, errors.New("unknown 'availability' value of technical asset '" + title + "': " + fmt.Sprintf("%v", asset.Availability))
		}

		dataFormatsAccepted := make([]types.DataFormat, 0)
		if asset.DataFormatsAccepted != nil {
			for _, dataFormatName := range asset.DataFormatsAccepted {
				dataFormat, err := types.ParseDataFormat(dataFormatName)
				if err != nil {
					panic(errors.New("unknown 'data_formats_accepted' value of technical asset '" + title + "': " + fmt.Sprintf("%v", dataFormatName)))
				}
				dataFormatsAccepted = append(dataFormatsAccepted, dataFormat)
			}
		}

		communicationLinks := make([]CommunicationLink, 0)
		if asset.CommunicationLinks != nil {
			for commLinkTitle, commLink := range asset.CommunicationLinks {
				constraint := true
				weight := 1
				var dataAssetsSent []string
				var dataAssetsReceived []string

				authentication, err := types.ParseAuthentication(commLink.Authentication)
				if err != nil {
					return nil, errors.New("unknown 'authentication' value of technical asset '" + title + "' communication link '" + commLinkTitle + "': " + fmt.Sprintf("%v", commLink.Authentication))
				}
				authorization, err := types.ParseAuthorization(commLink.Authorization)
				if err != nil {
					return nil, errors.New("unknown 'authorization' value of technical asset '" + title + "' communication link '" + commLinkTitle + "': " + fmt.Sprintf("%v", commLink.Authorization))
				}
				usage, err := types.ParseUsage(commLink.Usage)
				if err != nil {
					return nil, errors.New("unknown 'usage' value of technical asset '" + title + "' communication link '" + commLinkTitle + "': " + fmt.Sprintf("%v", commLink.Usage))
				}
				protocol, err := types.ParseProtocol(commLink.Protocol)
				if err != nil {
					return nil, errors.New("unknown 'protocol' value of technical asset '" + title + "' communication link '" + commLinkTitle + "': " + fmt.Sprintf("%v", commLink.Protocol))
				}

				if commLink.DataAssetsSent != nil {
					for _, dataAssetSent := range commLink.DataAssetsSent {
						referencedAsset := fmt.Sprintf("%v", dataAssetSent)
						err := parsedModel.checkDataAssetTargetExists(referencedAsset, "communication link '"+commLinkTitle+"' of technical asset '"+title+"'")
						if err != nil {
							return nil, err
						}
						dataAssetsSent = append(dataAssetsSent, referencedAsset)
					}
				}

				if commLink.DataAssetsReceived != nil {
					for _, dataAssetReceived := range commLink.DataAssetsReceived {
						referencedAsset := fmt.Sprintf("%v", dataAssetReceived)
						err := parsedModel.checkDataAssetTargetExists(referencedAsset, "communication link '"+commLinkTitle+"' of technical asset '"+title+"'")
						if err != nil {
							return nil, err
						}
						dataAssetsReceived = append(dataAssetsReceived, referencedAsset)
					}
				}

				if commLink.DiagramTweakWeight > 0 {
					weight = commLink.DiagramTweakWeight
				}

				constraint = !commLink.DiagramTweakConstraint

				dataFlowTitle := fmt.Sprintf("%v", commLinkTitle)
				if err != nil {
					return nil, err
				}
				commLinkId, err := createDataFlowId(id, dataFlowTitle)
				if err != nil {
					return nil, err
				}
				commLink := CommunicationLink{
					Id:                     commLinkId,
					SourceId:               id,
					TargetId:               commLink.Target,
					Title:                  dataFlowTitle,
					Description:            withDefault(commLink.Description, dataFlowTitle),
					Protocol:               protocol,
					Authentication:         authentication,
					Authorization:          authorization,
					Usage:                  usage,
					Tags:                   parsedModel.checkTags(lowerCaseAndTrim(commLink.Tags), "communication link '"+commLinkTitle+"' of technical asset '"+title+"'"),
					VPN:                    commLink.VPN,
					IpFiltered:             commLink.IpFiltered,
					Readonly:               commLink.Readonly,
					DataAssetsSent:         dataAssetsSent,
					DataAssetsReceived:     dataAssetsReceived,
					DiagramTweakWeight:     weight,
					DiagramTweakConstraint: constraint,
				}
				communicationLinks = append(communicationLinks, commLink)
				// track all comm links
				parsedModel.CommunicationLinks[commLink.Id] = commLink
				// keep track of map of *all* comm links mapped by target-id (to be able to look up "who is calling me" kind of things)
				parsedModel.IncomingTechnicalCommunicationLinksMappedByTargetId[commLink.TargetId] = append(
					parsedModel.IncomingTechnicalCommunicationLinksMappedByTargetId[commLink.TargetId], commLink)
			}
		}

		checkIdSyntax(id)
		if _, exists := parsedModel.TechnicalAssets[id]; exists {
			panic(errors.New("duplicate id used: " + id))
		}
		parsedModel.TechnicalAssets[id] = TechnicalAsset{
			Id:                      id,
			Usage:                   usage,
			Title:                   title, //fmt.Sprintf("%v", asset["title"]),
			Description:             withDefault(fmt.Sprintf("%v", asset.Description), title),
			Type:                    technicalAssetType,
			Size:                    technicalAssetSize,
			Technology:              technicalAssetTechnology,
			Tags:                    parsedModel.checkTags(lowerCaseAndTrim(asset.Tags), "technical asset '"+title+"'"),
			Machine:                 technicalAssetMachine,
			Internet:                asset.Internet,
			Encryption:              encryption,
			MultiTenant:             asset.MultiTenant,
			Redundant:               asset.Redundant,
			CustomDevelopedParts:    asset.CustomDevelopedParts,
			UsedAsClientByHuman:     asset.UsedAsClientByHuman,
			OutOfScope:              asset.OutOfScope,
			JustificationOutOfScope: fmt.Sprintf("%v", asset.JustificationOutOfScope),
			Owner:                   fmt.Sprintf("%v", asset.Owner),
			Confidentiality:         confidentiality,
			Integrity:               integrity,
			Availability:            availability,
			JustificationCiaRating:  fmt.Sprintf("%v", asset.JustificationCiaRating),
			DataAssetsProcessed:     dataAssetsProcessed,
			DataAssetsStored:        dataAssetsStored,
			DataFormatsAccepted:     dataFormatsAccepted,
			CommunicationLinks:      communicationLinks,
			DiagramTweakOrder:       asset.DiagramTweakOrder,
		}
	}

	// Trust Boundaries ===============================================================================
	checklistToAvoidAssetBeingModeledInMultipleTrustBoundaries := make(map[string]bool)
	parsedModel.TrustBoundaries = make(map[string]TrustBoundary)
	for title, boundary := range modelInput.TrustBoundaries {
		id := fmt.Sprintf("%v", boundary.ID)

		var technicalAssetsInside = make([]string, 0)
		if boundary.TechnicalAssetsInside != nil {
			parsedInsideAssets := boundary.TechnicalAssetsInside
			technicalAssetsInside = make([]string, len(parsedInsideAssets))
			for i, parsedInsideAsset := range parsedInsideAssets {
				technicalAssetsInside[i] = fmt.Sprintf("%v", parsedInsideAsset)
				_, found := parsedModel.TechnicalAssets[technicalAssetsInside[i]]
				if !found {
					panic(errors.New("missing referenced technical asset " + technicalAssetsInside[i] + " at trust boundary '" + title + "'"))
				}
				if checklistToAvoidAssetBeingModeledInMultipleTrustBoundaries[technicalAssetsInside[i]] == true {
					panic(errors.New("referenced technical asset " + technicalAssetsInside[i] + " at trust boundary '" + title + "' is modeled in multiple trust boundaries"))
				}
				checklistToAvoidAssetBeingModeledInMultipleTrustBoundaries[technicalAssetsInside[i]] = true
				//fmt.Println("asset "+technicalAssetsInside[i]+" at i="+strconv.Itoa(i))
			}
		}

		var trustBoundariesNested = make([]string, 0)
		if boundary.TrustBoundariesNested != nil {
			parsedNestedBoundaries := boundary.TrustBoundariesNested
			trustBoundariesNested = make([]string, len(parsedNestedBoundaries))
			for i, parsedNestedBoundary := range parsedNestedBoundaries {
				trustBoundariesNested[i] = fmt.Sprintf("%v", parsedNestedBoundary)
			}
		}

		trustBoundaryType, err := types.ParseTrustBoundary(boundary.Type)
		if err != nil {
			panic(errors.New("unknown 'type' of trust boundary '" + title + "': " + fmt.Sprintf("%v", boundary.Type)))
		}

		trustBoundary := TrustBoundary{
			Id:                    id,
			Title:                 title, //fmt.Sprintf("%v", boundary["title"]),
			Description:           withDefault(fmt.Sprintf("%v", boundary.Description), title),
			Type:                  trustBoundaryType,
			Tags:                  parsedModel.checkTags(lowerCaseAndTrim(boundary.Tags), "trust boundary '"+title+"'"),
			TechnicalAssetsInside: technicalAssetsInside,
			TrustBoundariesNested: trustBoundariesNested,
		}
		checkIdSyntax(id)
		if _, exists := parsedModel.TrustBoundaries[id]; exists {
			panic(errors.New("duplicate id used: " + id))
		}
		parsedModel.TrustBoundaries[id] = trustBoundary
		for _, technicalAsset := range trustBoundary.TechnicalAssetsInside {
			parsedModel.DirectContainingTrustBoundaryMappedByTechnicalAssetId[technicalAsset] = trustBoundary
			//fmt.Println("Asset "+technicalAsset+" is directly in trust boundary "+trustBoundary.Id)
		}
	}
	err = parsedModel.checkNestedTrustBoundariesExisting()
	if err != nil {
		return nil, err
	}

	// Shared Runtime ===============================================================================
	parsedModel.SharedRuntimes = make(map[string]SharedRuntime)
	for title, inputRuntime := range modelInput.SharedRuntimes {
		id := fmt.Sprintf("%v", inputRuntime.ID)

		var technicalAssetsRunning = make([]string, 0)
		if inputRuntime.TechnicalAssetsRunning != nil {
			parsedRunningAssets := inputRuntime.TechnicalAssetsRunning
			technicalAssetsRunning = make([]string, len(parsedRunningAssets))
			for i, parsedRunningAsset := range parsedRunningAssets {
				assetId := fmt.Sprintf("%v", parsedRunningAsset)
				err := parsedModel.CheckTechnicalAssetExists(assetId, "shared runtime '"+title+"'", false)
				if err != nil {
					return nil, err
				}
				technicalAssetsRunning[i] = assetId
			}
		}

		sharedRuntime := SharedRuntime{
			Id:                     id,
			Title:                  title, //fmt.Sprintf("%v", boundary["title"]),
			Description:            withDefault(fmt.Sprintf("%v", inputRuntime.Description), title),
			Tags:                   parsedModel.checkTags(inputRuntime.Tags, "shared runtime '"+title+"'"),
			TechnicalAssetsRunning: technicalAssetsRunning,
		}
		checkIdSyntax(id)
		if _, exists := parsedModel.SharedRuntimes[id]; exists {
			panic(errors.New("duplicate id used: " + id))
		}
		parsedModel.SharedRuntimes[id] = sharedRuntime
	}

	// Individual Risk Categories (just used as regular risk categories) ===============================================================================
	parsedModel.IndividualRiskCategories = make(map[string]RiskCategory)
	for title, individualCategory := range modelInput.IndividualRiskCategories {
		id := fmt.Sprintf("%v", individualCategory.ID)

		function, err := types.ParseRiskFunction(individualCategory.Function)
		if err != nil {
			panic(errors.New("unknown 'function' value of individual risk category '" + title + "': " + fmt.Sprintf("%v", individualCategory.Function)))
		}
		stride, err := types.ParseSTRIDE(individualCategory.STRIDE)
		if err != nil {
			panic(errors.New("unknown 'stride' value of individual risk category '" + title + "': " + fmt.Sprintf("%v", individualCategory.STRIDE)))
		}

		cat := RiskCategory{
			Id:                         id,
			Title:                      title,
			Description:                withDefault(fmt.Sprintf("%v", individualCategory.Description), title),
			Impact:                     fmt.Sprintf("%v", individualCategory.Impact),
			ASVS:                       fmt.Sprintf("%v", individualCategory.ASVS),
			CheatSheet:                 fmt.Sprintf("%v", individualCategory.CheatSheet),
			Action:                     fmt.Sprintf("%v", individualCategory.Action),
			Mitigation:                 fmt.Sprintf("%v", individualCategory.Mitigation),
			Check:                      fmt.Sprintf("%v", individualCategory.Check),
			DetectionLogic:             fmt.Sprintf("%v", individualCategory.DetectionLogic),
			RiskAssessment:             fmt.Sprintf("%v", individualCategory.RiskAssessment),
			FalsePositives:             fmt.Sprintf("%v", individualCategory.FalsePositives),
			Function:                   function,
			STRIDE:                     stride,
			ModelFailurePossibleReason: individualCategory.ModelFailurePossibleReason,
			CWE:                        individualCategory.CWE,
		}
		checkIdSyntax(id)
		if _, exists := parsedModel.IndividualRiskCategories[id]; exists {
			panic(errors.New("duplicate id used: " + id))
		}
		parsedModel.IndividualRiskCategories[id] = cat

		// NOW THE INDIVIDUAL RISK INSTANCES:
		//individualRiskInstances := make([]model.Risk, 0)
		if individualCategory.RisksIdentified != nil { // TODO: also add syntax checks of input YAML when linked asset is not found or when synthetic-id is already used...
			for title, individualRiskInstance := range individualCategory.RisksIdentified {
				var mostRelevantDataAssetId, mostRelevantTechnicalAssetId, mostRelevantCommunicationLinkId, mostRelevantTrustBoundaryId, mostRelevantSharedRuntimeId string
				var dataBreachProbability types.DataBreachProbability
				var dataBreachTechnicalAssetIDs []string
				severity, err := types.ParseRiskSeverity(individualRiskInstance.Severity)
				if err != nil {
					panic(errors.New("unknown 'severity' value of individual risk instance '" + title + "': " + fmt.Sprintf("%v", individualRiskInstance.Severity)))
				}
				exploitationLikelihood, err := types.ParseRiskExploitationLikelihood(individualRiskInstance.ExploitationLikelihood)
				if err != nil {
					panic(errors.New("unknown 'exploitation_likelihood' value of individual risk instance '" + title + "': " + fmt.Sprintf("%v", individualRiskInstance.ExploitationLikelihood)))
				}
				exploitationImpact, err := types.ParseRiskExploitationImpact(individualRiskInstance.ExploitationImpact)
				if err != nil {
					panic(errors.New("unknown 'exploitation_impact' value of individual risk instance '" + title + "': " + fmt.Sprintf("%v", individualRiskInstance.ExploitationImpact)))
				}

				if len(individualRiskInstance.MostRelevantDataAsset) > 0 {
					mostRelevantDataAssetId = fmt.Sprintf("%v", individualRiskInstance.MostRelevantDataAsset)
					err := parsedModel.checkDataAssetTargetExists(mostRelevantDataAssetId, "individual risk '"+title+"'")
					if err != nil {
						return nil, err
					}
				}

				if len(individualRiskInstance.MostRelevantTechnicalAsset) > 0 {
					mostRelevantTechnicalAssetId = fmt.Sprintf("%v", individualRiskInstance.MostRelevantTechnicalAsset)
					err := parsedModel.CheckTechnicalAssetExists(mostRelevantTechnicalAssetId, "individual risk '"+title+"'", false)
					if err != nil {
						return nil, err
					}
				}

				if len(individualRiskInstance.MostRelevantCommunicationLink) > 0 {
					mostRelevantCommunicationLinkId = fmt.Sprintf("%v", individualRiskInstance.MostRelevantCommunicationLink)
					err := parsedModel.checkCommunicationLinkExists(mostRelevantCommunicationLinkId, "individual risk '"+title+"'")
					if err != nil {
						return nil, err
					}
				}

				if len(individualRiskInstance.MostRelevantTrustBoundary) > 0 {
					mostRelevantTrustBoundaryId = fmt.Sprintf("%v", individualRiskInstance.MostRelevantTrustBoundary)
					err := parsedModel.checkTrustBoundaryExists(mostRelevantTrustBoundaryId, "individual risk '"+title+"'")
					if err != nil {
						return nil, err
					}
				}

				if len(individualRiskInstance.MostRelevantSharedRuntime) > 0 {
					mostRelevantSharedRuntimeId = fmt.Sprintf("%v", individualRiskInstance.MostRelevantSharedRuntime)
					err := parsedModel.checkSharedRuntimeExists(mostRelevantSharedRuntimeId, "individual risk '"+title+"'")
					if err != nil {
						return nil, err
					}
				}

				dataBreachProbability, err = types.ParseDataBreachProbability(individualRiskInstance.DataBreachProbability)
				if err != nil {
					return nil, errors.New("unknown 'data_breach_probability' value of individual risk instance '" + title + "': " + fmt.Sprintf("%v", individualRiskInstance.DataBreachProbability))
				}

				if individualRiskInstance.DataBreachTechnicalAssets != nil {
					dataBreachTechnicalAssetIDs = make([]string, len(individualRiskInstance.DataBreachTechnicalAssets))
					for i, parsedReferencedAsset := range individualRiskInstance.DataBreachTechnicalAssets {
						assetId := fmt.Sprintf("%v", parsedReferencedAsset)
						err := parsedModel.CheckTechnicalAssetExists(assetId, "data breach technical assets of individual risk '"+title+"'", false)
						if err != nil {
							return nil, err
						}
						dataBreachTechnicalAssetIDs[i] = assetId
					}
				}

				individualRiskInstance := Risk{
					SyntheticId:                     createSyntheticId(cat.Id, mostRelevantDataAssetId, mostRelevantTechnicalAssetId, mostRelevantCommunicationLinkId, mostRelevantTrustBoundaryId, mostRelevantSharedRuntimeId),
					Title:                           fmt.Sprintf("%v", title),
					Category:                        cat,
					Severity:                        severity,
					ExploitationLikelihood:          exploitationLikelihood,
					ExploitationImpact:              exploitationImpact,
					MostRelevantDataAssetId:         mostRelevantDataAssetId,
					MostRelevantTechnicalAssetId:    mostRelevantTechnicalAssetId,
					MostRelevantCommunicationLinkId: mostRelevantCommunicationLinkId,
					MostRelevantTrustBoundaryId:     mostRelevantTrustBoundaryId,
					MostRelevantSharedRuntimeId:     mostRelevantSharedRuntimeId,
					DataBreachProbability:           dataBreachProbability,
					DataBreachTechnicalAssetIDs:     dataBreachTechnicalAssetIDs,
				}
				parsedModel.GeneratedRisksByCategory[cat] = append(parsedModel.GeneratedRisksByCategory[cat], individualRiskInstance)
			}
		}
	}

	// Risk Tracking ===============================================================================
	parsedModel.RiskTracking = make(map[string]RiskTracking)
	for syntheticRiskId, riskTracking := range modelInput.RiskTracking {
		justification := fmt.Sprintf("%v", riskTracking.Justification)
		checkedBy := fmt.Sprintf("%v", riskTracking.CheckedBy)
		ticket := fmt.Sprintf("%v", riskTracking.Ticket)
		var date time.Time
		if len(riskTracking.Date) > 0 {
			var parseError error
			date, parseError = time.Parse("2006-01-02", riskTracking.Date)
			if parseError != nil {
				panic(errors.New("unable to parse 'date' of risk tracking '" + syntheticRiskId + "': " + riskTracking.Date))
			}
		}

		status, err := types.ParseRiskStatus(riskTracking.Status)
		if err != nil {
			panic(errors.New("unknown 'status' value of risk tracking '" + syntheticRiskId + "': " + riskTracking.Status))
		}

		tracking := RiskTracking{
			SyntheticRiskId: strings.TrimSpace(syntheticRiskId),
			Justification:   justification,
			CheckedBy:       checkedBy,
			Ticket:          ticket,
			Date:            date,
			Status:          status,
		}

		parsedModel.RiskTracking[syntheticRiskId] = tracking
	}

	// ====================== model consistency check (linking)
	for _, technicalAsset := range parsedModel.TechnicalAssets {
		for _, commLink := range technicalAsset.CommunicationLinks {
			err := parsedModel.CheckTechnicalAssetExists(commLink.TargetId, "communication link '"+commLink.Title+"' of technical asset '"+technicalAsset.Title+"'", false)
			if err != nil {
				return nil, err
			}
		}
	}

	return &parsedModel, nil
}

func checkIdSyntax(id string) {
	validIdSyntax := regexp.MustCompile(`^[a-zA-Z0-9\-]+$`)
	if !validIdSyntax.MatchString(id) {
		panic(errors.New("invalid id syntax used (only letters, numbers, and hyphen allowed): " + id))
	}
}

func createSyntheticId(categoryId string,
	mostRelevantDataAssetId, mostRelevantTechnicalAssetId, mostRelevantCommunicationLinkId, mostRelevantTrustBoundaryId, mostRelevantSharedRuntimeId string) string {
	result := categoryId
	if len(mostRelevantTechnicalAssetId) > 0 {
		result += "@" + mostRelevantTechnicalAssetId
	}
	if len(mostRelevantCommunicationLinkId) > 0 {
		result += "@" + mostRelevantCommunicationLinkId
	}
	if len(mostRelevantTrustBoundaryId) > 0 {
		result += "@" + mostRelevantTrustBoundaryId
	}
	if len(mostRelevantSharedRuntimeId) > 0 {
		result += "@" + mostRelevantSharedRuntimeId
	}
	if len(mostRelevantDataAssetId) > 0 {
		result += "@" + mostRelevantDataAssetId
	}
	return result
}

// in order to prevent Path-Traversal like stuff...
func removePathElementsFromImageFiles(overview input.Overview) input.Overview {
	for i := range overview.Images {
		newValue := make(map[string]string)
		for file, desc := range overview.Images[i] {
			newValue[filepath.Base(file)] = desc
		}
		overview.Images[i] = newValue
	}
	return overview
}

func withDefault(value string, defaultWhenEmpty string) string {
	trimmed := strings.TrimSpace(value)
	if len(trimmed) > 0 && trimmed != "<nil>" {
		return trimmed
	}
	return strings.TrimSpace(defaultWhenEmpty)
}

func lowerCaseAndTrim(tags []string) []string {
	for i := range tags {
		tags[i] = strings.ToLower(strings.TrimSpace(tags[i]))
	}
	return tags
}

func (parsedModel *ParsedModel) checkTags(tags []string, where string) []string {
	var tagsUsed = make([]string, 0)
	if tags != nil {
		tagsUsed = make([]string, len(tags))
		for i, parsedEntry := range tags {
			referencedTag := fmt.Sprintf("%v", parsedEntry)
			parsedModel.checkTagExists(referencedTag, where)
			tagsUsed[i] = referencedTag
		}
	}
	return tagsUsed
}

func (parsedModel *ParsedModel) checkTagExists(referencedTag, where string) {
	if !contains(parsedModel.TagsAvailable, referencedTag) {
		panic(errors.New("missing referenced tag in overall tag list at " + where + ": " + referencedTag))
	}
}

func createDataFlowId(sourceAssetId, title string) (string, error) {
	reg, err := regexp.Compile("[^A-Za-z0-9]+")
	if err != nil {
		return "", err
	}
	return sourceAssetId + ">" + strings.Trim(reg.ReplaceAllString(strings.ToLower(title), "-"), "- "), nil
}

func (parsedModel *ParsedModel) checkDataAssetTargetExists(referencedAsset, where string) error {
	if _, ok := parsedModel.DataAssets[referencedAsset]; !ok {
		panic(errors.New("missing referenced data asset target at " + where + ": " + referencedAsset))
	}
	return nil
}

func (parsedModel *ParsedModel) checkTrustBoundaryExists(referencedId, where string) error {
	if _, ok := parsedModel.TrustBoundaries[referencedId]; !ok {
		return errors.New("missing referenced trust boundary at " + where + ": " + referencedId)
	}
	return nil
}

func (parsedModel *ParsedModel) checkSharedRuntimeExists(referencedId, where string) error {
	if _, ok := parsedModel.SharedRuntimes[referencedId]; !ok {
		return errors.New("missing referenced shared runtime at " + where + ": " + referencedId)
	}
	return nil
}

func (parsedModel *ParsedModel) checkCommunicationLinkExists(referencedId, where string) error {
	if _, ok := parsedModel.CommunicationLinks[referencedId]; !ok {
		return errors.New("missing referenced communication link at " + where + ": " + referencedId)
	}
	return nil
}

func (parsedModel *ParsedModel) CheckTechnicalAssetExists(referencedAsset, where string, onlyForTweak bool) error {
	if _, ok := parsedModel.TechnicalAssets[referencedAsset]; !ok {
		suffix := ""
		if onlyForTweak {
			suffix = " (only referenced in diagram tweak)"
		}
		return errors.New("missing referenced technical asset target" + suffix + " at " + where + ": " + referencedAsset)
	}
	return nil
}

func (parsedModel *ParsedModel) checkNestedTrustBoundariesExisting() error {
	for _, trustBoundary := range parsedModel.TrustBoundaries {
		for _, nestedId := range trustBoundary.TrustBoundariesNested {
			if _, ok := parsedModel.TrustBoundaries[nestedId]; !ok {
				return errors.New("missing referenced nested trust boundary: " + nestedId)
			}
		}
	}
	return nil
}

func CalculateSeverity(likelihood types.RiskExploitationLikelihood, impact types.RiskExploitationImpact) types.RiskSeverity {
	result := likelihood.Weight() * impact.Weight()
	if result <= 1 {
		return types.LowSeverity
	}
	if result <= 3 {
		return types.MediumSeverity
	}
	if result <= 8 {
		return types.ElevatedSeverity
	}
	if result <= 12 {
		return types.HighSeverity
	}
	return types.CriticalSeverity
}

func (model *ParsedModel) InScopeTechnicalAssets() []TechnicalAsset {
	result := make([]TechnicalAsset, 0)
	for _, asset := range model.TechnicalAssets {
		if !asset.OutOfScope {
			result = append(result, asset)
		}
	}
	return result
}

func (what *ParsedModel) SortedTechnicalAssetIDs() []string {
	res := make([]string, 0)
	for id := range what.TechnicalAssets {
		res = append(res, id)
	}
	sort.Strings(res)
	return res
}

func (what *ParsedModel) TagsActuallyUsed() []string {
	result := make([]string, 0)
	for _, tag := range what.TagsAvailable {
		if len(what.TechnicalAssetsTaggedWithAny(tag)) > 0 ||
			len(what.CommunicationLinksTaggedWithAny(tag)) > 0 ||
			len(what.DataAssetsTaggedWithAny(tag)) > 0 ||
			len(what.TrustBoundariesTaggedWithAny(tag)) > 0 ||
			len(what.SharedRuntimesTaggedWithAny(tag)) > 0 {
			result = append(result, tag)
		}
	}
	return result
}

func (what *ParsedModel) TechnicalAssetsTaggedWithAny(tags ...string) []TechnicalAsset {
	result := make([]TechnicalAsset, 0)
	for _, candidate := range what.TechnicalAssets {
		if candidate.IsTaggedWithAny(tags...) {
			result = append(result, candidate)
		}
	}
	return result
}

func (what *ParsedModel) CommunicationLinksTaggedWithAny(tags ...string) []CommunicationLink {
	result := make([]CommunicationLink, 0)
	for _, asset := range what.TechnicalAssets {
		for _, candidate := range asset.CommunicationLinks {
			if candidate.IsTaggedWithAny(tags...) {
				result = append(result, candidate)
			}
		}
	}
	return result
}

func (what *ParsedModel) DataAssetsTaggedWithAny(tags ...string) []DataAsset {
	result := make([]DataAsset, 0)
	for _, candidate := range what.DataAssets {
		if candidate.IsTaggedWithAny(tags...) {
			result = append(result, candidate)
		}
	}
	return result
}

func (what *ParsedModel) TrustBoundariesTaggedWithAny(tags ...string) []TrustBoundary {
	result := make([]TrustBoundary, 0)
	for _, candidate := range what.TrustBoundaries {
		if candidate.IsTaggedWithAny(tags...) {
			result = append(result, candidate)
		}
	}
	return result
}

func (what *ParsedModel) SharedRuntimesTaggedWithAny(tags ...string) []SharedRuntime {
	result := make([]SharedRuntime, 0)
	for _, candidate := range what.SharedRuntimes {
		if candidate.IsTaggedWithAny(tags...) {
			result = append(result, candidate)
		}
	}
	return result
}

func (what *ParsedModel) OutOfScopeTechnicalAssets() []TechnicalAsset {
	assets := make([]TechnicalAsset, 0)
	for _, asset := range what.TechnicalAssets {
		if asset.OutOfScope {
			assets = append(assets, asset)
		}
	}
	sort.Sort(ByTechnicalAssetTitleSort(assets))
	return assets
}
