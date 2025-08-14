use anyhow::Result;
use rusqlite::{params, Connection};
use std::path::PathBuf;
use tauri::Manager;
use uuid::Uuid;

pub struct Db(pub Connection);

pub fn open_or_init(handle: &tauri::AppHandle) -> Result<Db> {
    let app_dir = handle
        .path()
        .app_data_dir()
        .expect("app data dir");
    std::fs::create_dir_all(&app_dir)?;
    let db_path = PathBuf::from(app_dir).join("chats.db");
    let conn = Connection::open(db_path)?;
    conn.execute_batch(
        r#"
        PRAGMA journal_mode=WAL;
        CREATE TABLE IF NOT EXISTS chats (
          id TEXT PRIMARY KEY,
          title TEXT NOT NULL,
          created_at INTEGER NOT NULL,
          updated_at INTEGER NOT NULL,
          model TEXT NOT NULL
        );
        CREATE TABLE IF NOT EXISTS messages (
          id TEXT PRIMARY KEY,
          chat_id TEXT NOT NULL,
          role TEXT NOT NULL,
          content TEXT NOT NULL,
          created_at INTEGER NOT NULL,
          FOREIGN KEY(chat_id) REFERENCES chats(id)
        );
        "#,
    )?;
    Ok(Db(conn))
}

pub fn create_chat(conn: &Connection, title: &str, model: &str) -> Result<String> {
    let id = Uuid::new_v4().to_string();
    let now = chrono::Utc::now().timestamp_millis();
    conn.execute(
        "INSERT INTO chats (id,title,created_at,updated_at,model) VALUES (?,?,?,?,?)",
        params![id, title, now, now, model],
    )?;
    Ok(id)
}

pub fn list_chats(conn: &Connection) -> Result<Vec<serde_json::Value>> {
    let mut stmt = conn.prepare("SELECT id,title,created_at,updated_at,model FROM chats ORDER BY updated_at DESC")?;
    let rows = stmt.query_map([], |row| {
        Ok(serde_json::json!({
            "id": row.get::<_, String>(0)?,
            "title": row.get::<_, String>(1)?,
            "created_at": row.get::<_, i64>(2)?,
            "updated_at": row.get::<_, i64>(3)?,
            "model": row.get::<_, String>(4)?,
        }))
    })?;
    Ok(rows.filter_map(|r| r.ok()).collect())
}

pub fn set_chat_model(conn: &Connection, chat_id: &str, model: &str) -> Result<()> {
    let now = chrono::Utc::now().timestamp_millis();
    conn.execute("UPDATE chats SET model=?, updated_at=? WHERE id=?", params![model, now, chat_id])?;
    Ok(())
}

pub fn add_message(conn: &Connection, chat_id: &str, role: &str, content: &str) -> Result<String> {
    let id = Uuid::new_v4().to_string();
    let now = chrono::Utc::now().timestamp_millis();
    conn.execute(
        "INSERT INTO messages (id,chat_id,role,content,created_at) VALUES (?,?,?,?,?)",
        params![id, chat_id, role, content, now],
    )?;
    conn.execute("UPDATE chats SET updated_at=? WHERE id=?", params![now, chat_id])?;
    Ok(id)
}

pub fn list_messages(conn: &Connection, chat_id: &str) -> Result<Vec<serde_json::Value>> {
    let mut stmt = conn.prepare("SELECT id,role,content,created_at FROM messages WHERE chat_id=? ORDER BY created_at ASC")?;
    let rows = stmt.query_map([chat_id], |row| {
        Ok(serde_json::json!({
            "id": row.get::<_, String>(0)?,
            "chat_id": chat_id,
            "role": row.get::<_, String>(1)?,
            "content": row.get::<_, String>(2)?,
            "created_at": row.get::<_, i64>(3)?,
        }))
    })?;
    Ok(rows.filter_map(|r| r.ok()).collect())
}
