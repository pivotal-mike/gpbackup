package backup

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/greenplum-db/gpbackup/utils"
)

/*
 * This file contains structs and functions related to backing up global cluster
 * metadata on the master that needs to be restored before data is restored,
 * such as roles and database configuration.
 */

/*
 * Session GUCs are printed to global, predata, and postdata files so we
 * will use the correct settings when the files are run during restore
 */
func PrintSessionGUCs(metadataFile *utils.FileWithByteCount, toc *utils.TOC, gucs SessionGUCs) {
	printUniversalSessionGUCs(metadataFile, toc, gucs)
	if connection.Version.Before("5") {
		print4OnlySessionGUCs(metadataFile, toc)
	}
}

func printUniversalSessionGUCs(metadataFile *utils.FileWithByteCount, toc *utils.TOC, gucs SessionGUCs) {
	start := metadataFile.ByteCount
	metadataFile.MustPrintf(`SET statement_timeout = 0;
SET check_function_bodies = false;
SET client_min_messages = error;
SET client_encoding = '%s';
SET standard_conforming_strings = on;
SET default_with_oids = %s;
`, gucs.ClientEncoding, gucs.DefaultWithOids)
	toc.AddMetadataEntry("", "", "SESSION GUCS", start, metadataFile)
}

/*
 * We print GPDB4-specific GUCs separately, with a different TOC entry type, so
 * that we can easily ignore them when restoring to a GPDB5 cluster.
 */
func print4OnlySessionGUCs(metadataFile *utils.FileWithByteCount, toc *utils.TOC) {
	start := metadataFile.ByteCount
	/*
	 * We always want to turn off strict XML parsing for restore, regardless of
	 * its current value.
	 */
	metadataFile.MustPrintf(`SET gp_strict_xml_parse = off;
`)
	toc.AddMetadataEntry("", "", "GPDB4 SESSION GUCS", start, metadataFile)
}

func PrintCreateDatabaseStatement(globalFile *utils.FileWithByteCount, toc *utils.TOC, db Database, dbMetadata MetadataMap) {
	dbname := db.Name
	start := globalFile.ByteCount
	globalFile.MustPrintf("\n\nCREATE DATABASE %s", dbname)
	if db.Tablespace != "pg_default" {
		globalFile.MustPrintf(" TABLESPACE %s", db.Tablespace)
	}
	globalFile.MustPrintf(";")
	toc.AddMetadataEntry("", dbname, "DATABASE", start, globalFile)
	start = globalFile.ByteCount
	PrintObjectMetadata(globalFile, dbMetadata[db.Oid], dbname, "DATABASE")
	if globalFile.ByteCount > start {
		toc.AddMetadataEntry("", dbname, "DATABASE METADATA", start, globalFile)
	}
}

func PrintDatabaseGUCs(globalFile *utils.FileWithByteCount, toc *utils.TOC, gucs []string, dbname string) {
	for _, guc := range gucs {
		start := globalFile.ByteCount
		globalFile.MustPrintf("\nALTER DATABASE %s %s;", dbname, guc)
		toc.AddMetadataEntry("", dbname, "DATABASE GUC", start, globalFile)
	}
}

func PrintCreateResourceQueueStatements(globalFile *utils.FileWithByteCount, toc *utils.TOC, resQueues []ResourceQueue, resQueueMetadata MetadataMap) {
	for _, resQueue := range resQueues {
		start := globalFile.ByteCount
		attributes := []string{}
		if resQueue.ActiveStatements != -1 {
			attributes = append(attributes, fmt.Sprintf("ACTIVE_STATEMENTS=%d", resQueue.ActiveStatements))
		}
		maxCostFloat, maxCostErr := strconv.ParseFloat(resQueue.MaxCost, 64)
		utils.CheckError(maxCostErr)
		if maxCostFloat > -1 {
			attributes = append(attributes, fmt.Sprintf("MAX_COST=%s", resQueue.MaxCost))
		}
		if resQueue.CostOvercommit {
			attributes = append(attributes, "COST_OVERCOMMIT=TRUE")
		}
		minCostFloat, minCostErr := strconv.ParseFloat(resQueue.MinCost, 64)
		utils.CheckError(minCostErr)
		if minCostFloat > 0 {
			attributes = append(attributes, fmt.Sprintf("MIN_COST=%s", resQueue.MinCost))
		}
		if resQueue.Priority != "medium" {
			attributes = append(attributes, fmt.Sprintf("PRIORITY=%s", strings.ToUpper(resQueue.Priority)))
		}
		if resQueue.MemoryLimit != "-1" {
			attributes = append(attributes, fmt.Sprintf("MEMORY_LIMIT='%s'", resQueue.MemoryLimit))
		}
		action := "CREATE"
		if resQueue.Name == "pg_default" {
			action = "ALTER"
		}
		globalFile.MustPrintf("\n\n%s RESOURCE QUEUE %s WITH (%s);", action, resQueue.Name, strings.Join(attributes, ", "))
		PrintObjectMetadata(globalFile, resQueueMetadata[resQueue.Oid], resQueue.Name, "RESOURCE QUEUE")
		toc.AddMetadataEntry("", resQueue.Name, "RESOURCE QUEUE", start, globalFile)
	}
}

type resGroupStruct struct {
	setting string
	value   int
}

func PrintCreateResourceGroupStatements(globalFile *utils.FileWithByteCount, toc *utils.TOC, resGroups []ResourceGroup, resGroupMetadata MetadataMap) {
	for _, resGroup := range resGroups {
		start := uint64(0)

		if resGroup.Name == "default_group" || resGroup.Name == "admin_group" {
			resGroupList := []resGroupStruct{
				{"CPU_RATE_LIMIT", resGroup.CPURateLimit},
				{"MEMORY_LIMIT", resGroup.MemoryLimit},
				{"MEMORY_SHARED_QUOTA", resGroup.MemorySharedQuota},
				{"MEMORY_SPILL_RATIO", resGroup.MemorySpillRatio},
				{"CONCURRENCY", resGroup.Concurrency},
			}
			for _, property := range resGroupList {
				start = globalFile.ByteCount
				globalFile.MustPrintf("\n\nALTER RESOURCE GROUP %s SET %s %d;", resGroup.Name, property.setting, property.value)
				PrintObjectMetadata(globalFile, resGroupMetadata[resGroup.Oid], resGroup.Name, "RESOURCE GROUP")
				toc.AddMetadataEntry("", resGroup.Name, "RESOURCE GROUP", start, globalFile)
			}
		} else {
			start = globalFile.ByteCount
			attributes := []string{}
			attributes = append(attributes, fmt.Sprintf("CPU_RATE_LIMIT=%d", resGroup.CPURateLimit))
			attributes = append(attributes, fmt.Sprintf("MEMORY_LIMIT=%d", resGroup.MemoryLimit))
			attributes = append(attributes, fmt.Sprintf("MEMORY_SHARED_QUOTA=%d", resGroup.MemorySharedQuota))
			attributes = append(attributes, fmt.Sprintf("MEMORY_SPILL_RATIO=%d", resGroup.MemorySpillRatio))
			attributes = append(attributes, fmt.Sprintf("CONCURRENCY=%d", resGroup.Concurrency))
			globalFile.MustPrintf("\n\nCREATE RESOURCE GROUP %s WITH (%s);", resGroup.Name, strings.Join(attributes, ", "))
			PrintObjectMetadata(globalFile, resGroupMetadata[resGroup.Oid], resGroup.Name, "RESOURCE GROUP")
			toc.AddMetadataEntry("", resGroup.Name, "RESOURCE GROUP", start, globalFile)
		}
	}
}

func PrintCreateRoleStatements(globalFile *utils.FileWithByteCount, toc *utils.TOC, roles []Role, roleMetadata MetadataMap) {
	for _, role := range roles {
		start := globalFile.ByteCount
		attrs := []string{}

		if role.Super {
			attrs = append(attrs, "SUPERUSER")
		} else {
			attrs = append(attrs, "NOSUPERUSER")
		}

		if role.Inherit {
			attrs = append(attrs, "INHERIT")
		} else {
			attrs = append(attrs, "NOINHERIT")
		}

		if role.CreateRole {
			attrs = append(attrs, "CREATEROLE")
		} else {
			attrs = append(attrs, "NOCREATEROLE")
		}

		if role.CreateDB {
			attrs = append(attrs, "CREATEDB")
		} else {
			attrs = append(attrs, "NOCREATEDB")
		}

		if role.CanLogin {
			attrs = append(attrs, "LOGIN")
		} else {
			attrs = append(attrs, "NOLOGIN")
		}
		if role.ConnectionLimit != -1 {
			attrs = append(attrs, fmt.Sprintf("CONNECTION LIMIT %d", role.ConnectionLimit))
		}

		if role.Password != "" {
			attrs = append(attrs, fmt.Sprintf("PASSWORD '%s'", role.Password))
		}

		if role.ValidUntil != "" {
			attrs = append(attrs, fmt.Sprintf("VALID UNTIL '%s'", role.ValidUntil))
		}

		attrs = append(attrs, fmt.Sprintf("RESOURCE QUEUE %s", role.ResQueue))

		if connection.Version.AtLeast("5") {
			attrs = append(attrs, fmt.Sprintf("RESOURCE GROUP %s", role.ResGroup))
		}

		if role.Createrexthttp {
			attrs = append(attrs, "CREATEEXTTABLE (protocol='http')")
		}

		if role.Createrextgpfd {
			attrs = append(attrs, "CREATEEXTTABLE (protocol='gpfdist', type='readable')")
		}

		if role.Createwextgpfd {
			attrs = append(attrs, "CREATEEXTTABLE (protocol='gpfdist', type='writable')")
		}

		if role.Createrexthdfs {
			attrs = append(attrs, "CREATEEXTTABLE (protocol='gphdfs', type='readable')")
		}

		if role.Createwexthdfs {
			attrs = append(attrs, "CREATEEXTTABLE (protocol='gphdfs', type='writable')")
		}

		globalFile.MustPrintf(`

CREATE ROLE %s;
ALTER ROLE %s WITH %s;`, role.Name, role.Name, strings.Join(attrs, " "))

		if len(role.TimeConstraints) != 0 {
			for _, timeConstraint := range role.TimeConstraints {
				globalFile.MustPrintf("\nALTER ROLE %s DENY BETWEEN DAY %d TIME '%s' AND DAY %d TIME '%s';", role.Name, timeConstraint.StartDay, timeConstraint.StartTime, timeConstraint.EndDay, timeConstraint.EndTime)
			}
		}
		PrintObjectMetadata(globalFile, roleMetadata[role.Oid], role.Name, "ROLE")
		toc.AddMetadataEntry("", role.Name, "ROLE", start, globalFile)
	}
}

func PrintRoleMembershipStatements(globalFile *utils.FileWithByteCount, toc *utils.TOC, roleMembers []RoleMember) {
	globalFile.MustPrintln("\n")
	for _, roleMember := range roleMembers {
		start := globalFile.ByteCount
		globalFile.MustPrintf("\nGRANT %s TO %s", roleMember.Role, roleMember.Member)
		if roleMember.IsAdmin {
			globalFile.MustPrintf(" WITH ADMIN OPTION")
		}
		globalFile.MustPrintf(" GRANTED BY %s;", roleMember.Grantor)
		toc.AddMetadataEntry("", roleMember.Member, "ROLE GRANT", start, globalFile)
	}
}

func PrintCreateTablespaceStatements(globalFile *utils.FileWithByteCount, toc *utils.TOC, tablespaces []Tablespace, tablespaceMetadata MetadataMap) {
	for _, tablespace := range tablespaces {
		start := globalFile.ByteCount
		globalFile.MustPrintf("\n\nCREATE TABLESPACE %s FILESPACE %s;", tablespace.Tablespace, tablespace.Filespace)
		PrintObjectMetadata(globalFile, tablespaceMetadata[tablespace.Oid], tablespace.Tablespace, "TABLESPACE")
		toc.AddMetadataEntry("", tablespace.Tablespace, "TABLESPACE", start, globalFile)
	}
}
