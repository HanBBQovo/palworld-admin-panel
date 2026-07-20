use std::collections::BTreeMap;
use std::env;
use std::error::Error;
use std::fs;
use std::path::{Path, PathBuf};

use psp_core::domain::guild::get_guild_details;
use psp_core::domain::player::get_player_details;
use psp_core::gamedata::GameData;
use psp_core::progress::null_progress;
use psp_core::session::{PlayerFileData, SaveKind, SaveSession};
use serde_json::{json, Map, Value};
use sha2::{Digest, Sha256};
use uuid::Uuid;

fn main() -> Result<(), Box<dyn Error>> {
    let mut args = env::args().skip(1);
    let snapshot_dir = PathBuf::from(args.next().ok_or("snapshot directory is required")?);
    let game_data_dir = PathBuf::from(args.next().ok_or("game data directory is required")?);

    let data = build_index(&snapshot_dir, &game_data_dir)?;
    println!("{}", serde_json::to_string(&data)?);
    Ok(())
}

fn build_index(snapshot_dir: &Path, game_data_dir: &Path) -> Result<Value, Box<dyn Error>> {
    let level_path = snapshot_dir.join("Level.sav");
    let players_dir = snapshot_dir.join("Players");
    if !level_path.is_file() || !players_dir.is_dir() {
        return Err("snapshot must contain Level.sav and Players/".into());
    }

    let player_refs = discover_player_refs(&players_dir)?;
    let level_bytes = fs::read(&level_path)?;
    let level_meta_path = snapshot_dir.join("LevelMeta.sav");
    let level_meta_bytes = level_meta_path
        .is_file()
        .then(|| fs::read(&level_meta_path))
        .transpose()?;
    let game_data = GameData::load(game_data_dir)?;
    let mut session = SaveSession::load(
        SaveKind::Steam {
            level_path: level_path.clone(),
        },
        snapshot_dir.to_string_lossy().into_owned(),
        "steam",
        &level_bytes,
        level_meta_bytes.as_deref(),
        None,
        player_refs,
        None,
        false,
        &null_progress(),
    )?;

    let player_ids = if session.player_summary_order.is_empty() {
        session.player_summaries.keys().copied().collect()
    } else {
        session.player_summary_order.clone()
    };
    let mut players = Vec::with_capacity(player_ids.len());
    let mut player_names = BTreeMap::new();
    for player_id in player_ids {
        let summary = session.player_summaries.get(&player_id).cloned();
        let detail = match get_player_details(&mut session, &game_data, player_id, &null_progress())
        {
            Ok(detail) => detail,
            Err(error) => {
                eprintln!("player {} detail failed: {}", uid_text(&player_id), error);
                None
            }
        };
        let row = player_row(summary.as_ref(), detail.as_ref());
        player_names.insert(
            uid_text(&player_id),
            row["nickname"].as_str().unwrap_or("").to_string(),
        );
        players.push(row);
    }

    let guild_ids = if session.guild_summary_order.is_empty() {
        session.guild_summaries.keys().copied().collect()
    } else {
        session.guild_summary_order.clone()
    };
    let mut guilds = Vec::with_capacity(guild_ids.len());
    for guild_id in guild_ids {
        let summary = session.guild_summaries.get(&guild_id).cloned();
        let detail = match get_guild_details(&mut session, &game_data, guild_id) {
            Ok(detail) => detail,
            Err(error) => {
                eprintln!("guild {} detail failed: {}", uid_text(&guild_id), error);
                None
            }
        };
        guilds.push(guild_row(summary.as_ref(), detail.as_ref(), &player_names));
    }

    players.sort_by(|left, right| {
        right["save_last_online"]
            .as_str()
            .cmp(&left["save_last_online"].as_str())
    });
    let fingerprint = snapshot_fingerprint(snapshot_dir)?;
    Ok(json!({
        "version": 2,
        "source": "Palworld Save Pal v1.2.0",
        "fingerprint": fingerprint,
        "updated_at": chrono_now(),
        "players": players,
        "guilds": guilds,
    }))
}

fn discover_player_refs(
    players_dir: &Path,
) -> Result<BTreeMap<Uuid, PlayerFileData>, Box<dyn Error>> {
    let mut refs = BTreeMap::new();
    for entry in fs::read_dir(players_dir)? {
        let path = entry?.path();
        if path.extension().and_then(|ext| ext.to_str()) != Some("sav") {
            continue;
        }
        let stem = path
            .file_stem()
            .and_then(|value| value.to_str())
            .unwrap_or("");
        let is_dps = stem.to_ascii_lowercase().ends_with("_dps");
        let raw_uid = if is_dps {
            &stem[..stem.len() - 4]
        } else {
            stem
        };
        let Ok(uid) = Uuid::parse_str(raw_uid) else {
            continue;
        };
        let entry = refs.entry(uid).or_insert(PlayerFileData::Paths {
            sav: None,
            dps: None,
        });
        if let PlayerFileData::Paths { sav, dps } = entry {
            if is_dps {
                *dps = Some(path);
            } else {
                *sav = Some(path);
            }
        }
    }
    Ok(refs)
}

fn snapshot_fingerprint(snapshot_dir: &Path) -> Result<String, Box<dyn Error>> {
    let mut paths = Vec::new();
    for name in ["Level.sav", "LevelMeta.sav", "WorldOption.sav"] {
        let path = snapshot_dir.join(name);
        if path.is_file() {
            paths.push(path);
        }
    }
    let players_dir = snapshot_dir.join("Players");
    if players_dir.is_dir() {
        for entry in fs::read_dir(players_dir)? {
            let path = entry?.path();
            if path.is_file()
                && path
                    .extension()
                    .and_then(|ext| ext.to_str())
                    .is_some_and(|ext| ext.eq_ignore_ascii_case("sav"))
            {
                paths.push(path);
            }
        }
    }
    paths.sort_by_key(|path| {
        path.strip_prefix(snapshot_dir)
            .unwrap_or(path)
            .to_string_lossy()
            .replace('\\', "/")
    });
    if paths.is_empty() {
        return Err("snapshot contains no indexable save files".into());
    }

    let mut hash = Sha256::new();
    for path in paths {
        let relative = path
            .strip_prefix(snapshot_dir)?
            .to_string_lossy()
            .replace('\\', "/");
        let bytes = fs::read(&path)?;
        hash.update(relative.as_bytes());
        hash.update([0]);
        hash.update((bytes.len() as u64).to_be_bytes());
        hash.update(bytes);
    }
    Ok(format!("{:x}", hash.finalize())[..16].to_string())
}

fn player_row(
    summary: Option<&psp_core::dto::summary::PlayerSummary>,
    detail: Option<&psp_core::dto::player::PlayerDto>,
) -> Value {
    let summary_name = summary
        .map(|item| item.nickname.clone())
        .unwrap_or_default();
    let summary_level = summary.and_then(|item| item.level).unwrap_or(0);
    let detail_json = detail.map(|item| serde_json::to_value(item).unwrap_or(Value::Null));
    let detail_json = detail_json.as_ref();
    let nickname = detail_json
        .and_then(|value| value.get("nickname"))
        .and_then(Value::as_str)
        .filter(|value| !value.is_empty())
        .unwrap_or(&summary_name);
    let level = detail_json
        .and_then(|value| value.get("level"))
        .and_then(Value::as_i64)
        .unwrap_or(summary_level);
    let location = detail_json.and_then(|value| value.get("location"));
    let summary_last_online = summary
        .and_then(|item| item.last_online_time.as_ref())
        .and_then(|value| serde_json::to_value(value).ok())
        .and_then(|value| value.as_str().map(str::to_string))
        .unwrap_or_default();
    let last_online = detail_json
        .and_then(|value| value.get("last_online_time"))
        .and_then(Value::as_str)
        .unwrap_or(&summary_last_online);
    let player_uid = detail
        .map(|item| uid_text(&item.uid))
        .or_else(|| summary.map(|item| uid_text(&item.uid)))
        .unwrap_or_default();

    let pals = detail_json
        .and_then(|value| value.get("pals"))
        .and_then(Value::as_object)
        .map(|values| {
            values
                .values()
                .map(|pal| {
                    json!({
                        "level": pal["level"],
                        "type": pal["character_id"],
                        "gender": pal["gender"],
                        "nickname": pal["nickname"].as_str().unwrap_or(""),
                        "is_lucky": pal["is_lucky"],
                        "is_boss": pal["is_boss"],
                        "workspeed": pal["rank_craftspeed"],
                        "melee": pal["rank_attack"],
                        "ranged": pal["talent_shot"],
                        "defense": pal["rank_defense"],
                        "skills": pal["active_skills"],
                    })
                })
                .collect::<Vec<_>>()
        })
        .unwrap_or_default();

    json!({
        "player_uid": player_uid,
        "nickname": nickname,
        "level": level,
        "exp": detail_json.and_then(|value| value["exp"].as_i64()).unwrap_or(0),
        "hp": detail_json.and_then(|value| value["hp"].as_i64()).unwrap_or(0),
        "max_hp": 0,
        "shield_hp": 0,
        "shield_max_hp": 0,
        "full_stomach": detail_json.and_then(|value| value["stomach"].as_f64()).unwrap_or(0.0),
        "save_last_online": last_online,
        "last_online": last_online,
        "steam_id": "",
        "user_id": "",
        "account_name": "",
        "ip": "",
        "ping": 0,
        "location_x": location.and_then(|value| value["x"].as_f64()).unwrap_or(0.0),
        "location_y": location.and_then(|value| value["y"].as_f64()).unwrap_or(0.0),
        "building_count": 0,
        "pals": pals,
        "items": item_rows(detail_json),
    })
}

fn item_rows(detail: Option<&Value>) -> Value {
    let mut result = Map::new();
    for (source, target) in [
        ("common_container", "CommonContainerId"),
        ("essential_container", "EssentialContainerId"),
        ("food_equip_container", "FoodEquipContainerId"),
        (
            "player_equipment_armor_container",
            "PlayerEquipArmorContainerId",
        ),
        ("weapon_load_out_container", "WeaponLoadOutContainerId"),
    ] {
        let rows = detail
            .and_then(|value| value.get(source))
            .and_then(|value| value.get("slots"))
            .and_then(Value::as_array)
            .map(|slots| {
                slots
                    .iter()
                    .map(|slot| {
                        let item_id = slot["static_id"]
                            .as_str()
                            .or_else(|| slot["dynamic_item"]["static_id"].as_str())
                            .unwrap_or("");
                        json!({
                            "SlotIndex": slot["slot_index"],
                            "ItemId": item_id,
                            "StackCount": slot["count"],
                        })
                    })
                    .collect::<Vec<_>>()
            })
            .unwrap_or_default();
        result.insert(target.to_string(), Value::Array(rows));
    }
    result.insert("DropSlotContainerId".to_string(), Value::Array(Vec::new()));
    Value::Object(result)
}

fn guild_row(
    summary: Option<&psp_core::dto::summary::GuildSummary>,
    detail: Option<&psp_core::dto::guild::GuildDto>,
    player_names: &BTreeMap<String, String>,
) -> Value {
    let summary_id = summary.map(|item| item.id);
    let detail_json = detail.map(|item| serde_json::to_value(item).unwrap_or(Value::Null));
    let detail_json = detail_json.as_ref();
    let players = detail_json
        .and_then(|value| value.get("players"))
        .and_then(Value::as_array)
        .map(|values| {
            values
                .iter()
                .filter_map(Value::as_str)
                .map(|uid| {
                    let key = uid.replace('-', "").to_uppercase();
                    json!({"player_uid": key, "nickname": player_names.get(&key).cloned().unwrap_or_default()})
                })
                .collect::<Vec<_>>()
        })
        .unwrap_or_default();
    let bases = detail_json
        .and_then(|value| value.get("bases"))
        .and_then(Value::as_object)
        .map(|values| {
            values
                .values()
                .map(|base| {
                    json!({
                        "id": base["id"].as_str().map(|value| value.replace('-', "").to_uppercase()).unwrap_or_default(),
                        "area": base["area_range"],
                        "location_x": base["location"]["x"],
                        "location_y": base["location"]["y"],
                    })
                })
                .collect::<Vec<_>>()
        })
        .unwrap_or_default();
    let admin = detail_json
        .and_then(|value| value["admin_player_uid"].as_str())
        .map(|value| value.replace('-', "").to_uppercase())
        .or_else(|| summary.and_then(|item| item.admin_player_uid.map(|value| uid_text(&value))));

    json!({
        "id": summary_id.map(|value| uid_text(&value)).unwrap_or_default(),
        "name": detail_json.and_then(|value| value["name"].as_str()).or_else(|| summary.map(|item| item.name.as_str())).unwrap_or(""),
        "base_camp_level": detail_json.and_then(|value| value["base_camp_level"].as_i64()).or_else(|| summary.and_then(|item| item.level)).unwrap_or(0),
        "admin_player_uid": admin.unwrap_or_default(),
        "players": players,
        "base_camp": bases,
    })
}

fn uid_text(value: &Uuid) -> String {
    value.simple().to_string().to_uppercase()
}

fn chrono_now() -> String {
    use std::time::{SystemTime, UNIX_EPOCH};
    let seconds = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap_or_default()
        .as_secs();
    format!("unix:{seconds}")
}
