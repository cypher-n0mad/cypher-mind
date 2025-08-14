#![allow(clippy::needless_return)]

mod db;
mod ollama;

use crate::db::*;
use crate::ollama::{list_models as ollama_list_models, stream_chat, ChatMessage};
use anyhow::Result;
use dashmap::DashMap;
use reqwest::Client;
use rusqlite::Connection;
use std::sync::Mutex;
use tauri::{Manager, State};
use tokio::task::JoinHandle;
use uuid::Uuid;

struct AppState {
    db: Mutex<Connection>,
    http: Client,
    base_url: String,
    streams: DashMap<String, JoinHandle<()>>, // stream_id -> task
}

#[tauri::command]
fn create_chat(state: State<AppState>, title: String, model: String) -> Result<String, String> {
    let conn = state.db.lock().unwrap();
    create_chat(&conn, &title, &model).map_err(|e| e.to_string())
}

#[tauri::command]
fn list_chats(state: State<AppState>) -> Result<Vec<serde_json::Value>, String> {
    let conn = state.db.lock().unwrap();
    list_chats(&conn).map_err(|e| e.to_string())
}

#[tauri::command]
fn set_chat_model(state: State<AppState>, chat_id: String, model: String) -> Result<(), String> {
    let conn = state.db.lock().unwrap();
    set_chat_model(&conn, &chat_id, &model).map_err(|e| e.to_string())
}

#[tauri::command]
fn add_message(state: State<AppState>, chat_id: String, role: String, content: String) -> Result<String, String> {
    let conn = state.db.lock().unwrap();
    add_message(&conn, &chat_id, &role, &content).map_err(|e| e.to_string())
}

#[tauri::command]
fn list_messages(state: State<AppState>, chat_id: String) -> Result<Vec<serde_json::Value>, String> {
    let conn = state.db.lock().unwrap();
    list_messages(&conn, &chat_id).map_err(|e| e.to_string())
}

#[tauri::command]
async fn list_models(state: State<'_, AppState>) -> Result<Vec<String>, String> {
    ollama_list_models(&state.http, &state.base_url)
        .await
        .map_err(|e| e.to_string())
}

#[tauri::command]
async fn chat_stream(app: tauri::AppHandle, state: State<'_, AppState>, chat_id: String) -> Result<String, String> {
    let conn = state.db.lock().unwrap();
    let msgs = list_messages(&conn, &chat_id).map_err(|e| e.to_string())?;
    // Build chat history for Ollama
    let messages: Vec<ChatMessage> = msgs
        .into_iter()
        .map(|m| ChatMessage {
            role: m.get("role").and_then(|v| v.as_str()).unwrap_or("").to_string(),
            content: m.get("content").and_then(|v| v.as_str()).unwrap_or("").to_string(),
        })
        .collect();

    // Load model from chat
    let row = {
        let mut stmt = conn.prepare("SELECT model FROM chats WHERE id=?").map_err(|e| e.to_string())?;
        let model: String = stmt.query_row([&chat_id], |r| r.get(0)).map_err(|e| e.to_string())?;
        model
    };
    drop(conn);

    let stream_id = Uuid::new_v4().to_string();
    let base = state.base_url.clone();
    let client = state.http.clone();

    let handle = tauri::async_runtime::spawn({
        let app = app.clone();
        let chat_id_clone = chat_id.clone();
        async move {
            match stream_chat(&client, &base, &row, messages).await {
                Ok(mut stream) => {
                    use futures_util::StreamExt;
                    while let Some(chunk) = stream.next().await {
                        match chunk {
                            Ok(s) => {
                                for line in s.split('\n') {
                                    if line.trim().is_empty() { continue; }
                                    if let Ok(v) = serde_json::from_str::<serde_json::Value>(line) {
                                        if let Some(content) = v.get("message").and_then(|m| m.get("content")).and_then(|c| c.as_str()) {
                                            let _ = app.emit_all(&format!("chat-token:{}", chat_id_clone), content.to_string());
                                        }
                                        if v.get("done").and_then(|d| d.as_bool()).unwrap_or(false) {
                                            let _ = app.emit_all(&format!("chat-done:{}", chat_id_clone), ());
                                        }
                                    }
                                }
                            },
                            Err(e) => {
                                let _ = app.emit_all(&format!("chat-token:{}", chat_id_clone), format!("[error: {}]", e));
                                break;
                            }
                        }
                    }
                }
                Err(e) => {
                    let _ = app.emit_all(&format!("chat-token:{}", chat_id_clone), format!("[error: {}]", e));
                }
            }
        }
    });

    state.streams.insert(stream_id.clone(), handle);
    Ok(stream_id)
}

#[tauri::command]
async fn cancel_stream(state: State<'_, AppState>, stream_id: String) -> Result<(), String> {
    if let Some((_, handle)) = state.streams.remove(&stream_id) { handle.abort(); }
    Ok(())
}

fn main() {
    tauri::Builder::default()
        .setup(|app| {
            let db = open_or_init(app).expect("db init").0;
            let client = Client::builder().build().unwrap();
            app.manage(AppState {
                db: Mutex::new(db),
                http: client,
                base_url: "http://localhost:11434".to_string(),
                streams: DashMap::new(),
            });
            Ok(())
        })
        .invoke_handler(tauri::generate_handler![
            create_chat,
            list_chats,
            set_chat_model,
            add_message,
            list_messages,
            list_models,
            chat_stream,
            cancel_stream,
        ])
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}
