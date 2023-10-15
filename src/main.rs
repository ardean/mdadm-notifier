mod config;

use std::process::Command;
use std::str::FromStr;

use config::Config;
use dotenv::dotenv;
use serenity::async_trait;
use serenity::model::prelude::{ChannelId, Ready};
use serenity::prelude::*;

struct Handler;

#[async_trait]
impl EventHandler for Handler {
    async fn ready(&self, context: Context, ready: Ready) {
        println!("{} is connected!", ready.user.name);

        let raid_info = get_raid_info().await;
        send_message(&context, &raid_info).await;
    }
}

#[tokio::main]
async fn main() {
    dotenv().ok();

    let config = config::create();
    let intents = GatewayIntents::DIRECT_MESSAGES;
    let mut client = Client::builder(config.discord.token.clone(), intents)
        .event_handler(Handler)
        .await
        .expect("Error creating client");

    {
        let mut data = client.data.write().await;
        data.insert::<Config>(config);
    }

    if let Err(why) = client.start().await {
        println!("An error occurred while running the client: {:?}", why);
    }
}

async fn send_message(context: &Context, message: &str) {
    println!("sending: {}", message);

    let data = context.data.read().await;
    let config = data.get::<Config>().unwrap();
    let channel_id_str = config.discord.channel_id.as_str();
    let channel_id = ChannelId::from_str(channel_id_str).expect("channel id invalid");

    if let Err(why) = channel_id.say(context, message).await {
        println!("Error sending message: {:?}", why);
    }
}

async fn get_raid_info() -> String {
    let command = "mdadm";
    let args = &["-D", "/dev/md0"];

    return run_command(command, args).await;
}

async fn run_command(command: &str, args: &[&str]) -> String {
    let output = Command::new(command)
        .args(args)
        .output()
        .expect("Failed to execute command");

    let value: String;
    if output.status.success() {
        value = String::from_utf8_lossy(&output.stdout).to_string();
    } else {
        value = String::from_utf8_lossy(&output.stderr).to_string();
    }

    return value;
}
