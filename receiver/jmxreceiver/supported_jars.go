// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package jmxreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/jmxreceiver"

import "fmt"

type supportedJar struct {
	jar             string
	version         string
	addedValidation func(c *Config, j supportedJar) error
}

// Provided as a build time variable if a development or customer specific JMX Metrics Gatherer needs to be supported
var MetricsGathererHash = "n/a"

// Support for SSL properties passed via property file will be available starting in v1.14.0
func oldFormatProperties(c *Config, j supportedJar) error {
	if c.KeystorePassword != "" ||
		c.KeystorePath != "" ||
		c.KeystoreType != "" ||
		c.TruststorePassword != "" ||
		c.TruststorePath != "" ||
		c.TruststoreType != "" {
		return fmt.Errorf("version %s of the JMX Metrics Gatherer does not support SSL parameters (Keystore & Truststore) "+
			"from the jmxreceiver. Update to the latest JMX Metrics Gatherer if you would like SSL support", j.version)
	}
	return nil
}

// If you change this variable name, please open an issue in opentelemetry-java-contrib
// so that repository's release automation can be updated
var jmxMetricsGathererVersions = map[string]supportedJar{
	"e0fc9b92364413ae33d1c33b5927e4eead70d6fab7ca626b56649947352636b4": {
		version: "1.45.0-alpha",
		jar:     "JMX metrics gatherer",
	},
	"67a415eace3d513c3c5bf30518a9035c48b14e7c8ad43b0ddb572588de7b4ce6": {
		version: "1.44.0-alpha",
		jar:     "JMX metrics gatherer",
	},
	"6c1d4c82d76f2826acf43981ef0b222f55eea841aebcc604a0daafbb2bddb93c": {
		version: "1.43.0-alpha",
		jar:     "JMX metrics gatherer",
	},
	"e19041d478c2f3641cee499bae74baa66c97c193b0012369deeb587d5add958a": {
		version: "1.42.0-alpha",
		jar:     "JMX metrics gatherer",
	},
	"8005bee5861f0a9f72577ee6e64d2f9f7ce72a063c88ba38db9568785c7f0cfd": {
		version: "1.41.0-alpha",
		jar:     "JMX metrics gatherer",
	},
	"a51b50329446ae7516888ef915e4b20fb61b986b2230d66eacaf61d8690525c9": {
		version: "1.40.0-alpha",
		jar:     "JMX metrics gatherer",
	},
	"43543679b12c7772fffe78ad9dcde3421cb5dd2a704231f9901d32578b2cf69e": {
		version: "1.39.0-alpha",
		jar:     "JMX metrics gatherer",
	},
	"75d03922df2306086b9eee2daccbfd8c9b94ce140a482fb4698a839ec3d3f192": {
		version: "1.38.0-alpha",
		jar:     "JMX metrics gatherer",
	},
	"74d23e8714deab770c940a14175ab5dfd0cd0c16e198861e45a72fbb854bc727": {
		version: "1.37.0-alpha",
		jar:     "JMX metrics gatherer",
	},
	"ab53c54b7cd8a6e31bb40e8ab3a9a9dedc9386cb4905c2a7f2188d3baae99f39": {
		version: "1.36.0-alpha",
		jar:     "JMX metrics gatherer",
	},
	"60b2ee1a798c35d10f6e3602ea46f1b1c0298080262636d73b4fc652b7dcd0da": {
		version: "1.35.0-alpha",
		jar:     "JMX metrics gatherer",
	},
	"a939251bbdb91ede2b5fbe891fd50775dd21f41c3369b5abec7dd74e4bf1a9fd": {
		version: "1.34.0-alpha",
		jar:     "JMX metrics gatherer",
	},
	"50ad8ed45fa17bc6edafe4649008d1f0b57181d3162e64c76d6da9a49272db33": {
		version: "1.33.0-alpha",
		jar:     "JMX metrics gatherer",
	},
	"0ba6647cc31b627dbe20de87d696c3cffb0f72f0fc2ad3d3fe2be8aa3582bf26": {
		version: "1.32.0-alpha",
		jar:     "JMX metrics gatherer",
	},
	"9f4f7ab6fe7040dbeb91fae75dc17199c408a6339eb498d2f29088b78c621c2c": {
		version: "1.31.0-alpha",
		jar:     "JMX metrics gatherer",
	},
	"0b4b280fa745702e83a0b3c0d191144c9069c215848c61d3092d41f000770e12": {
		version: "1.30.0-alpha",
		jar:     "JMX metrics gatherer",
	},
	"0b947c255f3fd358343ab43475875dbb09233d369ff91a88a28c38f767a2a6fb": {
		version: "1.29.0-alpha",
		jar:     "JMX metrics gatherer",
	},
	"32fead1e233e67dea56f0d08628540938a41ecd87a3b4c4bdf78193c3b62c6dd": {
		version: "1.28.0-alpha",
		jar:     "JMX metrics gatherer",
	},
	"97d3a0767642297d7259ac274c4eb107b4e83d48fa2b8d91ceb800a31437a734": {
		version: "1.27.0-alpha",
		jar:     "JMX metrics gatherer",
	},
	"12e6dba902d35872cd69b99f23863dc9536660620fc0eb9eed8d0e45b2354970": {
		version: "1.26.0-alpha",
		jar:     "JMX metrics gatherer",
	},
	"6a41aa8fb8edcafac604324818216a405a96245255a66ba96cf3668ef59927b8": {
		version: "1.25.1-alpha",
		jar:     "JMX metrics gatherer",
	},
	"b6f5030acdbef44afb79afe1c547f55a446f6e60c24db9cdcf6e8dba49f87a16": {
		version: "1.25.0-alpha",
		jar:     "JMX metrics gatherer",
	},
	"7ac5485801bf5fea347aac0d828bade875686fecbed83b3ce44088c87bdf9d46": {
		version: "1.24.0-alpha",
		jar:     "JMX metrics gatherer",
	},
	"af15e12aa1edb0f694408cbf0b7fae7fb8d36e7904d9b68c93c7645101769f63": {
		version: "1.23.0-alpha",
		jar:     "JMX metrics gatherer",
	},
	"b90f675c5722931d2ebbb4bae959841b78fe5f87fe461a23a8837f95dec517ff": {
		version: "1.22.0-alpha",
		jar:     "JMX metrics gatherer",
	},
	"ca176a2cb59884f4436139587ff891e47160449354dcbc9b7b71ed26d4185962": {
		version: "1.21.0-alpha",
		jar:     "JMX metrics gatherer",
	},
	"7150bd5f2d594aa9fff9572d5aefeed5ca9d6465d2c967befe00cae6a1b13416": {
		version: "1.20.1-alpha",
		jar:     "JMX metrics gatherer",
	},
	"ef4267c2ff607200c40a87233eb7a3c6457ffaa190463faa83fdcc331d6161d8": {
		version: "1.20.0-alpha",
		jar:     "JMX metrics gatherer",
	},
	"3c46cff8521cdb0d36bb2891b15cbc1bb2fcbca7c5344253403ab30fe9f693a6": {
		version: "1.15.0-alpha",
		jar:     "JMX metrics gatherer",
	},
	"0646639df98404bd9b1263b46e2fd4612bc378f9951a561f0a0be9725718db36": {
		version: "1.14.0-alpha",
		jar:     "JMX metrics gatherer",
	},
	"623572be30e3c546d60b0ac890935790bc3cb8d0b4ff5150a58b43a99f68ed05": {
		version:         "1.13.0-alpha",
		jar:             "JMX metrics gatherer",
		addedValidation: oldFormatProperties,
	},
	"c0b1a19c4965c7961abaaccfbb4d358e5f3b0b5b105578a4782702f126bfa8b7": {
		version:         "1.12.0-alpha",
		jar:             "JMX metrics gatherer",
		addedValidation: oldFormatProperties,
	},
	"ca689ca2da8a412c7f4ea0e816f47e8639b4270a48fb877c9a910b44757bc0a4": {
		version:         "1.11.0-alpha",
		jar:             "JMX metrics gatherer",
		addedValidation: oldFormatProperties,
	},
	"4b14d26fb383ed925fe1faf1b7fe2103559ed98ce6cf761ac9afc0158d2a218c": {
		version:         "1.10.0-alpha",
		jar:             "JMX metrics gatherer",
		addedValidation: oldFormatProperties,
	},
}

// Separated into two functions for tests
func init() {
	initSupportedJars()
}

func initSupportedJars() {
	if MetricsGathererHash != "n/a" {
		jmxMetricsGathererVersions = map[string]supportedJar{
			MetricsGathererHash: {
				version: "custom",
				jar:     "JMX metrics gatherer",
			},
		}
	}
}

var wildflyJarVersions = map[string]supportedJar{
	"637d78e6c2275178623012e75e407b7e36856e26f05bd8eebc68a79628eaf6e4": {
		version: "9.0.2",
		jar:     "wildfly jboss client",
	},
	"7e695092dae15744f787a1765aed65819f04cd7f3f3cc88c38f6004d1acfb65e": {
		version: "10.1.0",
		jar:     "wildfly jboss client",
	},
	"c46e38bdcc9601614c0ef6c7fea5d72b8369eee8cd1501eb691abfc9c0743eac": {
		version: "11.0.0",
		jar:     "wildfly jboss client",
	},
	"1cd7e1f40b9e8023d80c79986e54253576f52fdcc4837ea9d59f9f0c2aae6b75": {
		version: "12.0.0",
		jar:     "wildfly jboss client",
	},
	"ea0cb65bbca5f9948244f8a6a3c8c7d2aba39703ca518064aeb54dce8c618947": {
		version: "13.0.0",
		jar:     "wildfly jboss client",
	},
	"3eba02a6300f635c87a48a8450f3a83637c28f8fd9970c24c4e3637d6702a8b1": {
		version: "14.0.0",
		jar:     "wildfly jboss client",
	},
	"01cc95f4344f31da95aeee81427eef9038ad0982f2b1c9288e30dea03ea649c7": {
		version: "14.0.1",
		jar:     "wildfly jboss client",
	},
	"f3f5af42ba64a32b95b5790737e086286a737c7cdce2b9935748b5b74212f399": {
		version: "15.0.0",
		jar:     "wildfly jboss client",
	},
	"dcc6549d8a09164748b5e58ecbba28a60930952ac5e2e2d4e23db42a177e3056": {
		version: "15.0.1",
		jar:     "wildfly jboss client",
	},
	"20e09860bca8446f8969b0f0076b4639a9633b3aa9b5cf1bb6199ed38f0c20ce": {
		version: "16.0.0",
		jar:     "wildfly jboss client",
	},
	"c766d9ab833f7e32be82d4e815a73fb3d0b97bcf22a81ea46135a46e18158ea7": {
		version: "17.0.0",
		jar:     "wildfly jboss client",
	},
	"49a47c0636886d3c9fab14c026c1271236be4032ed9d2bb77c0326efdefa7f7f": {
		version: "17.0.1",
		jar:     "wildfly jboss client",
	},
	"a439a4058a935714ad7a7d2c743573a42852a9b3c52e7810931fb18df20b3a9f": {
		version: "18.0.0",
		jar:     "wildfly jboss client",
	},
	"9f4d9cdbec1d2ec613fea2f455d543cf203347307fc11167d056fabe8816f441": {
		version: "18.0.1",
		jar:     "wildfly jboss client",
	},
	"e94f6d22332ad2e658883cee6b03940a73acf22d5655f9544f335ce018a65895": {
		version: "19.0.0",
		jar:     "wildfly jboss client",
	},
	"08d8445dd74665119eca7329f6e84dafcf6557e1e3c06e99db4ffa76029d2942": {
		version: "19.1.0",
		jar:     "wildfly jboss client",
	},
	"43a7faa5919aced0089be5f1810336fccfdb9f470e2b05f0fc32aa66455687e4": {
		version: "20.0.0",
		jar:     "wildfly jboss client",
	},
	"4b35baa7164c9447edfc418bd8b41ec2c6bd7069c80ae32bdee659c1243582c0": {
		version: "20.0.1",
		jar:     "wildfly jboss client",
	},
	"038842dc04a50b87fecea01d3449c4e03d8364c26c07624779dc47832d807369": {
		version: "21.0.0",
		jar:     "wildfly jboss client",
	},
	"3a8e301a3ef7ba443648102e0bc4b31ea6bacba8fe0832a0210ae548274cfb94": {
		version: "21.0.1",
		jar:     "wildfly jboss client",
	},
	"740cf15531c73b6b11ad022f3cd401b9cf28e80ec36f2130ecaac08fba5109ba": {
		version: "21.0.2",
		jar:     "wildfly jboss client",
	},
	"06bec7ed175fa65b76f554c5a989a53fecf412cb71adeeb5731833fbc211ff53": {
		version: "22.0.0",
		jar:     "wildfly jboss client",
	},
	"39daed5a4f73b173b822988ea161dcfae37b459984d67cb71fc29c7e0c33873c": {
		version: "22.0.1",
		jar:     "wildfly jboss client",
	},
	"6d41c7f3ba33cbcfb1a5a639eccc1d04c494f855f15324e175736c4ac184336d": {
		version: "23.0.0",
		jar:     "wildfly jboss client",
	},
	"8f974f36a927b8a51f2ef807c58e10960bc502c1caa5b93722e3dc913c74c466": {
		version: "23.0.1",
		jar:     "wildfly jboss client",
	},
	"fb9b638b04f0e54adc8343daed0fb07d38deadcc7ee8ba48a1c3ac44d9f87cff": {
		version: "23.0.2",
		jar:     "wildfly jboss client",
	},
	"fb633ed945d21de548b266f09b9467295571e429a301eae456713424cbc23464": {
		version: "24.0.0",
		jar:     "wildfly jboss client",
	},
	"cd5b72bbdbb99123a78d9837339f10a849f6c48d8503840cf9ab673cae4039b6": {
		version: "24.0.1",
		jar:     "wildfly jboss client",
	},
	"56d1c33707c38860192d4678cecdb4041198a97b98cd852c8d3a4ffa23133977": {
		version: "25.0.0",
		jar:     "wildfly jboss client",
	},
	"b160673ab755d82b423cc41be07df7922c3481e74b7ad324e664460f2c24341c": {
		version: "25.0.1",
		jar:     "wildfly jboss client",
	},
	"132a8d51d5ab3a5394257501b70a059850af7b4b3c8037cf5caedd3c50c17bc0": {
		version: "26.0.0",
		jar:     "wildfly jboss client",
	},
	"859e69844bf047193f4ab461c7cc2b7c4b18ec76d1f32ebebcb9470097ef7dcd": {
		version: "26.0.1",
		jar:     "wildfly jboss client",
	},
	"86b65d22d3904e6fa7a9016e574b67ad1caf0548d7bc51b229f9026ed459738f": {
		version: "26.1.0",
		jar:     "wildfly jboss client",
	},
	"8dd73d59bc458457f95abb2532d24650ac1025b6150fa7d4a24c674ab309eb02": {
		version: "26.1.1",
		jar:     "wildfly jboss client",
	},
}
