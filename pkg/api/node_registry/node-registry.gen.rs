// Example code that deserializes and serializes the model.
// extern crate serde;
// #[macro_use]
// extern crate serde_derive;
// extern crate serde_json;
//
// use generated_module::NodeInfo;
//
// fn main() {
//     let json = r#"{"answer": 42}"#;
//     let model: NodeInfo = serde_json::from_str(&json).unwrap();
// }

use serde::{Serialize, Deserialize};
use std::collections::HashMap;

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct RegisterRequest {
    pub id: String,

    pub info: NodeInfo,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct NodeInfo {
    pub connectors: HashMap<String, ConnectorInfo>,

    pub ips: Vec<String>,

    pub presets: Vec<String>,

    pub public_key: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ConnectorInfo {
    pub data: String,

    pub protocol: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct RegisterResponse {
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct GetRequest {
    pub id: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct GetResponse {
    pub found: bool,

    pub info: NodeInfo,
}
