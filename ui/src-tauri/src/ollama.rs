use anyhow::{anyhow, Result};
use futures_util::StreamExt;
use reqwest::Client;
use serde::{Deserialize, Serialize};
use serde_json::json;
use std::time::Duration;

#[derive(Debug, Deserialize)]
pub struct Tag { pub name: String }

pub async fn list_models(client: &Client, base: &str) -> Result<Vec<String>> {
    let url = format!("{}/api/tags", base);
    let res = client.get(url).timeout(Duration::from_secs(10)).send().await?;
    #[derive(Deserialize)]
    struct Tags { models: Vec<Tag> }
    let t: Tags = res.json().await?;
    Ok(t.models.into_iter().map(|m| m.name).collect())
}

#[derive(Debug, Serialize, Deserialize, Clone)]
pub struct ChatMessage { pub role: String, pub content: String }

pub async fn stream_chat(
    client: &Client,
    base: &str,
    model: &str,
    messages: Vec<ChatMessage>,
) -> Result<impl futures_util::Stream<Item = Result<String>>> {
    let url = format!("{}/api/chat", base);
    let body = json!({
        "model": model,
        "messages": messages,
        "stream": true
    });
    let res = client
        .post(url)
        .json(&body)
        .send()
        .await?;

    if !res.status().is_success() {
        return Err(anyhow!("ollama returned status {}", res.status()));
    }

    let stream = res.bytes_stream().map(|chunk| {
        let chunk = chunk.map_err(|e| anyhow!(e))?;
        let s = String::from_utf8_lossy(&chunk).to_string();
        Ok(s)
    });

    Ok(stream)
}
