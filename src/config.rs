use serde::Deserialize;
use serenity::prelude::TypeMapKey;
use std::env;

#[derive(Deserialize)]
pub struct Config {
    pub discord: DiscordConfig,
}

impl TypeMapKey for Config {
    type Value = Config;
}

#[derive(Deserialize)]
pub struct DiscordConfig {
    pub token: String,
    pub channel_id: String,
}

pub fn create() -> Config {
    Config {
        discord: DiscordConfig {
            token: env::var("DISCORD_TOKEN").expect("DISCORD_TOKEN is missing"),
            channel_id: env::var("DISCORD_CHANNEL_ID").expect("DISCORD_CHANNEL_ID is missing"),
        },
    }
}
