// Copyright 2022 The AmazeChain Authors
// This file is part of the AmazeChain library.
//
// The AmazeChain library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The AmazeChain library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the AmazeChain library. If not, see <http://www.gnu.org/licenses/>.

package conf

type MetricsConfig struct {
	Enable bool   `json:"enable" yaml:"enable"`
	Port   int    `json:"port" yaml:"port"`
	HTTP   string `json:"http" yaml:"http"`

	EnableInfluxDB       bool   `json:"enable_influx_db" yaml:"enable_influx_db"`
	InfluxDBEndpoint     string `json:"influx_db_endpoint" yaml:"influx_db_endpoint"`
	InfluxDBDatabase     string `json:"influx_db_database" yaml:"influx_db_database"`
	InfluxDBUsername     string `json:"influx_db_username" yaml:"influx_db_username"`
	InfluxDBPassword     string `json:"influx_db_password" yaml:"influx_db_password"`
	InfluxDBToken        string `json:"influx_db_token" yaml:"influx_db_token"`
	InfluxDBBucket       string `json:"influx_db_bucket" yaml:"influx_db_bucket"`
	InfluxDBOrganization string `json:"influx_db_organization" yaml:"influx_db_organization"`
	InfluxDBTags         string `json:"influx_db_tags" yaml:"influx_db_tags"`
}
