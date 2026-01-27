from typing import List, Dict, Any, Optional
from mcp.server.fastmcp import FastMCP
from whatsapp import (
    # Existing functions
    search_contacts as whatsapp_search_contacts,
    list_messages as whatsapp_list_messages,
    list_chats as whatsapp_list_chats,
    get_chat as whatsapp_get_chat,
    get_direct_chat_by_contact as whatsapp_get_direct_chat_by_contact,
    get_contact_chats as whatsapp_get_contact_chats,
    get_last_interaction as whatsapp_get_last_interaction,
    get_message_context as whatsapp_get_message_context,
    send_message as whatsapp_send_message,
    send_file as whatsapp_send_file,
    send_audio_message as whatsapp_audio_voice_message,
    download_media as whatsapp_download_media,
    # Message features
    react_to_message as whatsapp_react_to_message,
    reply_to_message as whatsapp_reply_to_message,
    delete_message as whatsapp_delete_message,
    edit_message as whatsapp_edit_message,
    # Chat management
    mark_chat_read as whatsapp_mark_chat_read,
    # Group management
    create_group as whatsapp_create_group,
    get_group_info as whatsapp_get_group_info,
    add_group_members as whatsapp_add_group_members,
    remove_group_members as whatsapp_remove_group_members,
    promote_group_admin as whatsapp_promote_group_admin,
    demote_group_admin as whatsapp_demote_group_admin,
    set_group_name as whatsapp_set_group_name,
    set_group_topic as whatsapp_set_group_topic,
    set_group_photo as whatsapp_set_group_photo,
    get_group_invite_link as whatsapp_get_group_invite_link,
    revoke_group_invite as whatsapp_revoke_group_invite,
    join_group_via_invite as whatsapp_join_group_via_invite,
    leave_group as whatsapp_leave_group,
    # Profile & Privacy
    get_profile as whatsapp_get_profile,
    set_profile_photo as whatsapp_set_profile_photo,
    get_privacy_settings as whatsapp_get_privacy_settings,
    block_contact as whatsapp_block_contact,
    unblock_contact as whatsapp_unblock_contact,
    get_blocked_contacts as whatsapp_get_blocked_contacts,
    # Presence
    subscribe_presence as whatsapp_subscribe_presence,
    send_typing_indicator as whatsapp_send_typing_indicator,
    send_recording_indicator as whatsapp_send_recording_indicator,
    set_presence_online as whatsapp_set_presence_online,
    set_presence_offline as whatsapp_set_presence_offline,
    # Status/Stories
    post_text_status as whatsapp_post_text_status,
    post_image_status as whatsapp_post_image_status,
    # Utilities
    send_location as whatsapp_send_location,
    send_contact_card as whatsapp_send_contact_card,
    check_phone_numbers as whatsapp_check_phone_numbers,
    get_contact_info as whatsapp_get_contact_info,
    # Chat state management (pin, archive, mute, star)
    pin_chat as whatsapp_pin_chat,
    unpin_chat as whatsapp_unpin_chat,
    archive_chat as whatsapp_archive_chat,
    unarchive_chat as whatsapp_unarchive_chat,
    mute_chat as whatsapp_mute_chat,
    unmute_chat as whatsapp_unmute_chat,
    delete_chat as whatsapp_delete_chat,
    star_message as whatsapp_star_message,
    unstar_message as whatsapp_unstar_message,
    forward_message as whatsapp_forward_message,
    label_chat as whatsapp_label_chat,
)

# Initialize FastMCP server
mcp = FastMCP("whatsapp")

# ============== EXISTING TOOLS ==============

@mcp.tool()
def search_contacts(query: str) -> List[Dict[str, Any]]:
    """Search WhatsApp contacts by name or phone number.

    Args:
        query: Search term to match against contact names or phone numbers
    """
    contacts = whatsapp_search_contacts(query)
    return contacts

@mcp.tool()
def list_messages(
    after: Optional[str] = None,
    before: Optional[str] = None,
    sender_phone_number: Optional[str] = None,
    chat_jid: Optional[str] = None,
    query: Optional[str] = None,
    limit: int = 20,
    page: int = 0,
    include_context: bool = True,
    context_before: int = 1,
    context_after: int = 1
) -> List[Dict[str, Any]]:
    """Get WhatsApp messages matching specified criteria with optional context.

    Args:
        after: Optional ISO-8601 formatted string to only return messages after this date
        before: Optional ISO-8601 formatted string to only return messages before this date
        sender_phone_number: Optional phone number to filter messages by sender
        chat_jid: Optional chat JID to filter messages by chat
        query: Optional search term to filter messages by content
        limit: Maximum number of messages to return (default 20)
        page: Page number for pagination (default 0)
        include_context: Whether to include messages before and after matches (default True)
        context_before: Number of messages to include before each match (default 1)
        context_after: Number of messages to include after each match (default 1)
    """
    messages = whatsapp_list_messages(
        after=after,
        before=before,
        sender_phone_number=sender_phone_number,
        chat_jid=chat_jid,
        query=query,
        limit=limit,
        page=page,
        include_context=include_context,
        context_before=context_before,
        context_after=context_after
    )
    return messages

@mcp.tool()
def list_chats(
    query: Optional[str] = None,
    limit: int = 20,
    page: int = 0,
    include_last_message: bool = True,
    sort_by: str = "last_active"
) -> List[Dict[str, Any]]:
    """Get WhatsApp chats matching specified criteria.

    Args:
        query: Optional search term to filter chats by name or JID
        limit: Maximum number of chats to return (default 20)
        page: Page number for pagination (default 0)
        include_last_message: Whether to include the last message in each chat (default True)
        sort_by: Field to sort results by, either "last_active" or "name" (default "last_active")
    """
    chats = whatsapp_list_chats(
        query=query,
        limit=limit,
        page=page,
        include_last_message=include_last_message,
        sort_by=sort_by
    )
    return chats

@mcp.tool()
def get_chat(chat_jid: str, include_last_message: bool = True) -> Dict[str, Any]:
    """Get WhatsApp chat metadata by JID.

    Args:
        chat_jid: The JID of the chat to retrieve
        include_last_message: Whether to include the last message (default True)
    """
    chat = whatsapp_get_chat(chat_jid, include_last_message)
    return chat

@mcp.tool()
def get_direct_chat_by_contact(sender_phone_number: str) -> Dict[str, Any]:
    """Get WhatsApp chat metadata by sender phone number.

    Args:
        sender_phone_number: The phone number to search for
    """
    chat = whatsapp_get_direct_chat_by_contact(sender_phone_number)
    return chat

@mcp.tool()
def get_contact_chats(jid: str, limit: int = 20, page: int = 0) -> List[Dict[str, Any]]:
    """Get all WhatsApp chats involving the contact.

    Args:
        jid: The contact's JID to search for
        limit: Maximum number of chats to return (default 20)
        page: Page number for pagination (default 0)
    """
    chats = whatsapp_get_contact_chats(jid, limit, page)
    return chats

@mcp.tool()
def get_last_interaction(jid: str) -> str:
    """Get most recent WhatsApp message involving the contact.

    Args:
        jid: The JID of the contact to search for
    """
    message = whatsapp_get_last_interaction(jid)
    return message

@mcp.tool()
def get_message_context(
    message_id: str,
    before: int = 5,
    after: int = 5
) -> Dict[str, Any]:
    """Get context around a specific WhatsApp message.

    Args:
        message_id: The ID of the message to get context for
        before: Number of messages to include before the target message (default 5)
        after: Number of messages to include after the target message (default 5)
    """
    context = whatsapp_get_message_context(message_id, before, after)
    return context

@mcp.tool()
def send_message(
    recipient: str,
    message: str
) -> Dict[str, Any]:
    """Send a WhatsApp message to a person or group. For group chats use the JID.

    Args:
        recipient: The recipient - either a phone number with country code but no + or other symbols,
                 or a JID (e.g., "123456789@s.whatsapp.net" or a group JID like "123456789@g.us")
        message: The message text to send

    Returns:
        A dictionary containing success status and a status message
    """
    if not recipient:
        return {"success": False, "message": "Recipient must be provided"}

    success, status_message = whatsapp_send_message(recipient, message)
    return {"success": success, "message": status_message}

@mcp.tool()
def send_file(recipient: str, media_path: str) -> Dict[str, Any]:
    """Send a file such as a picture, raw audio, video or document via WhatsApp to the specified recipient.

    Args:
        recipient: The recipient - either a phone number with country code but no + or other symbols,
                 or a JID (e.g., "123456789@s.whatsapp.net" or a group JID like "123456789@g.us")
        media_path: The absolute path to the media file to send (image, video, document)

    Returns:
        A dictionary containing success status and a status message
    """
    success, status_message = whatsapp_send_file(recipient, media_path)
    return {"success": success, "message": status_message}

@mcp.tool()
def send_audio_message(recipient: str, media_path: str) -> Dict[str, Any]:
    """Send any audio file as a WhatsApp voice message to the specified recipient.

    Args:
        recipient: The recipient - phone number or JID
        media_path: The absolute path to the audio file (will be converted to Opus .ogg if needed)

    Returns:
        A dictionary containing success status and a status message
    """
    success, status_message = whatsapp_audio_voice_message(recipient, media_path)
    return {"success": success, "message": status_message}

@mcp.tool()
def download_media(message_id: str, chat_jid: str) -> Dict[str, Any]:
    """Download media from a WhatsApp message and get the local file path.

    Args:
        message_id: The ID of the message containing the media
        chat_jid: The JID of the chat containing the message

    Returns:
        A dictionary containing success status, a status message, and the file path if successful
    """
    file_path = whatsapp_download_media(message_id, chat_jid)

    if file_path:
        return {"success": True, "message": "Media downloaded successfully", "file_path": file_path}
    else:
        return {"success": False, "message": "Failed to download media"}


# ============== MESSAGE FEATURES ==============

@mcp.tool()
def react_to_message(chat_jid: str, message_id: str, sender: str, emoji: str) -> Dict[str, Any]:
    """React to a WhatsApp message with an emoji.

    Args:
        chat_jid: The JID of the chat containing the message
        message_id: The ID of the message to react to
        sender: The JID of the original message sender
        emoji: The emoji to react with (empty string to remove reaction)

    Returns:
        A dictionary containing success status and a status message
    """
    success, status_message = whatsapp_react_to_message(chat_jid, message_id, sender, emoji)
    return {"success": success, "message": status_message}

@mcp.tool()
def reply_to_message(
    chat_jid: str,
    message_id: str,
    quoted_sender: str,
    message: str,
    media_path: Optional[str] = None
) -> Dict[str, Any]:
    """Reply to a specific WhatsApp message.

    Args:
        chat_jid: The JID of the chat containing the message
        message_id: The ID of the message to reply to
        quoted_sender: The JID of the sender of the message being replied to
        message: The reply message text
        media_path: Optional path to media file to include in reply

    Returns:
        A dictionary containing success status and a status message
    """
    success, status_message = whatsapp_reply_to_message(chat_jid, message_id, quoted_sender, message, media_path)
    return {"success": success, "message": status_message}

@mcp.tool()
def delete_message(chat_jid: str, message_id: str, sender: str, for_all: bool = True) -> Dict[str, Any]:
    """Delete a WhatsApp message.

    Args:
        chat_jid: The JID of the chat containing the message
        message_id: The ID of the message to delete
        sender: The JID of the message sender
        for_all: Whether to delete for everyone (True) or just for me (False)

    Returns:
        A dictionary containing success status and a status message
    """
    success, status_message = whatsapp_delete_message(chat_jid, message_id, sender, for_all)
    return {"success": success, "message": status_message}

@mcp.tool()
def edit_message(chat_jid: str, message_id: str, new_text: str) -> Dict[str, Any]:
    """Edit a sent WhatsApp message.

    Args:
        chat_jid: The JID of the chat containing the message
        message_id: The ID of the message to edit
        new_text: The new message text

    Returns:
        A dictionary containing success status and a status message
    """
    success, status_message = whatsapp_edit_message(chat_jid, message_id, new_text)
    return {"success": success, "message": status_message}


# ============== CHAT MANAGEMENT ==============

@mcp.tool()
def mark_chat_read(chat_jid: str, message_ids: List[str], sender: str) -> Dict[str, Any]:
    """Mark WhatsApp messages as read.

    Args:
        chat_jid: The JID of the chat
        message_ids: List of message IDs to mark as read
        sender: The JID of the message sender

    Returns:
        A dictionary containing success status and a status message
    """
    success, status_message = whatsapp_mark_chat_read(chat_jid, message_ids, sender)
    return {"success": success, "message": status_message}


# ============== GROUP MANAGEMENT ==============

@mcp.tool()
def create_group(name: str, participants: List[str]) -> Dict[str, Any]:
    """Create a new WhatsApp group.

    Args:
        name: The name for the new group
        participants: List of participant JIDs or phone numbers to add

    Returns:
        A dictionary containing success status, message, and group_jid if successful
    """
    success, status_message, group_jid = whatsapp_create_group(name, participants)
    result = {"success": success, "message": status_message}
    if group_jid:
        result["group_jid"] = group_jid
    return result

@mcp.tool()
def get_group_info(group_jid: str) -> Dict[str, Any]:
    """Get information about a WhatsApp group.

    Args:
        group_jid: The JID of the group

    Returns:
        A dictionary containing group info (name, topic, participants, admins, etc.)
    """
    return whatsapp_get_group_info(group_jid)

@mcp.tool()
def add_group_members(group_jid: str, participants: List[str]) -> Dict[str, Any]:
    """Add members to a WhatsApp group.

    Args:
        group_jid: The JID of the group
        participants: List of participant JIDs or phone numbers to add

    Returns:
        A dictionary containing success status and a status message
    """
    success, status_message = whatsapp_add_group_members(group_jid, participants)
    return {"success": success, "message": status_message}

@mcp.tool()
def remove_group_members(group_jid: str, participants: List[str]) -> Dict[str, Any]:
    """Remove members from a WhatsApp group.

    Args:
        group_jid: The JID of the group
        participants: List of participant JIDs to remove

    Returns:
        A dictionary containing success status and a status message
    """
    success, status_message = whatsapp_remove_group_members(group_jid, participants)
    return {"success": success, "message": status_message}

@mcp.tool()
def promote_group_admin(group_jid: str, participants: List[str]) -> Dict[str, Any]:
    """Promote members to admin in a WhatsApp group.

    Args:
        group_jid: The JID of the group
        participants: List of participant JIDs to promote

    Returns:
        A dictionary containing success status and a status message
    """
    success, status_message = whatsapp_promote_group_admin(group_jid, participants)
    return {"success": success, "message": status_message}

@mcp.tool()
def demote_group_admin(group_jid: str, participants: List[str]) -> Dict[str, Any]:
    """Demote admins to regular members in a WhatsApp group.

    Args:
        group_jid: The JID of the group
        participants: List of participant JIDs to demote

    Returns:
        A dictionary containing success status and a status message
    """
    success, status_message = whatsapp_demote_group_admin(group_jid, participants)
    return {"success": success, "message": status_message}

@mcp.tool()
def set_group_name(group_jid: str, name: str) -> Dict[str, Any]:
    """Set the name of a WhatsApp group.

    Args:
        group_jid: The JID of the group
        name: The new group name

    Returns:
        A dictionary containing success status and a status message
    """
    success, status_message = whatsapp_set_group_name(group_jid, name)
    return {"success": success, "message": status_message}

@mcp.tool()
def set_group_topic(group_jid: str, topic: str) -> Dict[str, Any]:
    """Set the description/topic of a WhatsApp group.

    Args:
        group_jid: The JID of the group
        topic: The new group description

    Returns:
        A dictionary containing success status and a status message
    """
    success, status_message = whatsapp_set_group_topic(group_jid, topic)
    return {"success": success, "message": status_message}

@mcp.tool()
def set_group_photo(group_jid: str, photo_path: str) -> Dict[str, Any]:
    """Set the photo of a WhatsApp group.

    Args:
        group_jid: The JID of the group
        photo_path: The absolute path to the photo file

    Returns:
        A dictionary containing success status and a status message
    """
    success, status_message = whatsapp_set_group_photo(group_jid, photo_path)
    return {"success": success, "message": status_message}

@mcp.tool()
def get_group_invite_link(group_jid: str) -> Dict[str, Any]:
    """Get the invite link for a WhatsApp group.

    Args:
        group_jid: The JID of the group

    Returns:
        A dictionary containing success status, message, and invite_link if successful
    """
    success, status_message, invite_link = whatsapp_get_group_invite_link(group_jid)
    result = {"success": success, "message": status_message}
    if invite_link:
        result["invite_link"] = invite_link
    return result

@mcp.tool()
def revoke_group_invite(group_jid: str) -> Dict[str, Any]:
    """Revoke the current group invite link and get a new one.

    Args:
        group_jid: The JID of the group

    Returns:
        A dictionary containing success status, message, and new invite_link if successful
    """
    success, status_message, invite_link = whatsapp_revoke_group_invite(group_jid)
    result = {"success": success, "message": status_message}
    if invite_link:
        result["invite_link"] = invite_link
    return result

@mcp.tool()
def join_group_via_invite(invite_link: str) -> Dict[str, Any]:
    """Join a WhatsApp group using an invite link.

    Args:
        invite_link: The group invite link (full URL or just the code)

    Returns:
        A dictionary containing success status, message, and group_jid if successful
    """
    success, status_message, group_jid = whatsapp_join_group_via_invite(invite_link)
    result = {"success": success, "message": status_message}
    if group_jid:
        result["group_jid"] = group_jid
    return result

@mcp.tool()
def leave_group(group_jid: str) -> Dict[str, Any]:
    """Leave a WhatsApp group.

    Args:
        group_jid: The JID of the group to leave

    Returns:
        A dictionary containing success status and a status message
    """
    success, status_message = whatsapp_leave_group(group_jid)
    return {"success": success, "message": status_message}


# ============== PROFILE & PRIVACY ==============

@mcp.tool()
def get_profile() -> Dict[str, Any]:
    """Get the current user's WhatsApp profile information.

    Returns:
        A dictionary containing profile info (jid, phone_number, etc.)
    """
    return whatsapp_get_profile()

@mcp.tool()
def set_profile_photo(photo_path: str) -> Dict[str, Any]:
    """Set the current user's WhatsApp profile photo.

    Args:
        photo_path: The absolute path to the photo file

    Returns:
        A dictionary containing success status and a status message
    """
    success, status_message = whatsapp_set_profile_photo(photo_path)
    return {"success": success, "message": status_message}

@mcp.tool()
def get_privacy_settings() -> Dict[str, Any]:
    """Get the current WhatsApp privacy settings.

    Returns:
        A dictionary containing privacy settings (last_seen, profile_photo, about, etc.)
    """
    return whatsapp_get_privacy_settings()

@mcp.tool()
def block_contact(jid: str) -> Dict[str, Any]:
    """Block a WhatsApp contact.

    Args:
        jid: The JID of the contact to block

    Returns:
        A dictionary containing success status and a status message
    """
    success, status_message = whatsapp_block_contact(jid)
    return {"success": success, "message": status_message}

@mcp.tool()
def unblock_contact(jid: str) -> Dict[str, Any]:
    """Unblock a WhatsApp contact.

    Args:
        jid: The JID of the contact to unblock

    Returns:
        A dictionary containing success status and a status message
    """
    success, status_message = whatsapp_unblock_contact(jid)
    return {"success": success, "message": status_message}

@mcp.tool()
def get_blocked_contacts() -> Dict[str, Any]:
    """Get the list of blocked WhatsApp contacts.

    Returns:
        A dictionary containing success status, message, and list of blocked contacts
    """
    return whatsapp_get_blocked_contacts()


# ============== PRESENCE ==============

@mcp.tool()
def subscribe_presence(jid: str) -> Dict[str, Any]:
    """Subscribe to presence updates for a WhatsApp contact.

    Args:
        jid: The JID of the contact to subscribe to

    Returns:
        A dictionary containing success status and a status message
    """
    success, status_message = whatsapp_subscribe_presence(jid)
    return {"success": success, "message": status_message}

@mcp.tool()
def send_typing_indicator(chat_jid: str, typing: bool = True) -> Dict[str, Any]:
    """Send a typing indicator to a WhatsApp chat.

    Args:
        chat_jid: The JID of the chat
        typing: True to show typing, False to stop

    Returns:
        A dictionary containing success status and a status message
    """
    success, status_message = whatsapp_send_typing_indicator(chat_jid, typing)
    return {"success": success, "message": status_message}

@mcp.tool()
def send_recording_indicator(chat_jid: str, recording: bool = True) -> Dict[str, Any]:
    """Send a recording indicator to a WhatsApp chat.

    Args:
        chat_jid: The JID of the chat
        recording: True to show recording, False to stop

    Returns:
        A dictionary containing success status and a status message
    """
    success, status_message = whatsapp_send_recording_indicator(chat_jid, recording)
    return {"success": success, "message": status_message}

@mcp.tool()
def set_presence_online() -> Dict[str, Any]:
    """Set WhatsApp presence to online.

    Returns:
        A dictionary containing success status and a status message
    """
    success, status_message = whatsapp_set_presence_online()
    return {"success": success, "message": status_message}

@mcp.tool()
def set_presence_offline() -> Dict[str, Any]:
    """Set WhatsApp presence to offline.

    Returns:
        A dictionary containing success status and a status message
    """
    success, status_message = whatsapp_set_presence_offline()
    return {"success": success, "message": status_message}


# ============== STATUS/STORIES ==============

@mcp.tool()
def post_text_status(text: str) -> Dict[str, Any]:
    """Post a text status/story to WhatsApp.

    Args:
        text: The text content for the status

    Returns:
        A dictionary containing success status and a status message
    """
    success, status_message = whatsapp_post_text_status(text)
    return {"success": success, "message": status_message}

@mcp.tool()
def post_image_status(media_path: str, caption: Optional[str] = None) -> Dict[str, Any]:
    """Post an image status/story to WhatsApp.

    Args:
        media_path: The absolute path to the image file
        caption: Optional caption for the image

    Returns:
        A dictionary containing success status and a status message
    """
    success, status_message = whatsapp_post_image_status(media_path, caption)
    return {"success": success, "message": status_message}


# ============== UTILITIES ==============

@mcp.tool()
def send_location(
    chat_jid: str,
    latitude: float,
    longitude: float,
    name: Optional[str] = None,
    address: Optional[str] = None
) -> Dict[str, Any]:
    """Send a location message to a WhatsApp chat.

    Args:
        chat_jid: The JID of the chat
        latitude: The latitude coordinate
        longitude: The longitude coordinate
        name: Optional name for the location
        address: Optional address for the location

    Returns:
        A dictionary containing success status and a status message
    """
    success, status_message = whatsapp_send_location(chat_jid, latitude, longitude, name, address)
    return {"success": success, "message": status_message}

@mcp.tool()
def send_contact_card(chat_jid: str, display_name: str, phone_number: str) -> Dict[str, Any]:
    """Send a contact card to a WhatsApp chat.

    Args:
        chat_jid: The JID of the chat
        display_name: The display name for the contact
        phone_number: The phone number for the contact (with country code, no + or symbols)

    Returns:
        A dictionary containing success status and a status message
    """
    success, status_message = whatsapp_send_contact_card(chat_jid, display_name, phone_number)
    return {"success": success, "message": status_message}

@mcp.tool()
def check_phone_numbers(phone_numbers: List[str]) -> Dict[str, Any]:
    """Check if phone numbers are registered on WhatsApp.

    Args:
        phone_numbers: List of phone numbers to check (with country code)

    Returns:
        A dictionary containing success status, message, results (phone -> is_registered), and jids (phone -> jid)
    """
    return whatsapp_check_phone_numbers(phone_numbers)

@mcp.tool()
def get_contact_info_by_jid(jid: str) -> Dict[str, Any]:
    """Get information about a WhatsApp contact.

    Args:
        jid: The JID of the contact

    Returns:
        A dictionary containing contact info (name, full_name, push_name, business_name, etc.)
    """
    return whatsapp_get_contact_info(jid)


# ============== CHAT STATE TOOLS (PIN, ARCHIVE, MUTE, STAR) ==============

@mcp.tool()
def pin_chat(chat_jid: str) -> Dict[str, Any]:
    """Pin a WhatsApp chat to the top of the chat list.

    Args:
        chat_jid: The JID of the chat to pin

    Returns:
        A dictionary with success status and message
    """
    success, message = whatsapp_pin_chat(chat_jid)
    return {"success": success, "message": message}


@mcp.tool()
def unpin_chat(chat_jid: str) -> Dict[str, Any]:
    """Unpin a WhatsApp chat.

    Args:
        chat_jid: The JID of the chat to unpin

    Returns:
        A dictionary with success status and message
    """
    success, message = whatsapp_unpin_chat(chat_jid)
    return {"success": success, "message": message}


@mcp.tool()
def archive_chat(chat_jid: str) -> Dict[str, Any]:
    """Archive a WhatsApp chat. Archiving also unpins the chat.

    Args:
        chat_jid: The JID of the chat to archive

    Returns:
        A dictionary with success status and message
    """
    success, message = whatsapp_archive_chat(chat_jid)
    return {"success": success, "message": message}


@mcp.tool()
def unarchive_chat(chat_jid: str) -> Dict[str, Any]:
    """Unarchive a WhatsApp chat.

    Args:
        chat_jid: The JID of the chat to unarchive

    Returns:
        A dictionary with success status and message
    """
    success, message = whatsapp_unarchive_chat(chat_jid)
    return {"success": success, "message": message}


@mcp.tool()
def mute_chat(chat_jid: str, duration_seconds: int = 0) -> Dict[str, Any]:
    """Mute a WhatsApp chat.

    Args:
        chat_jid: The JID of the chat to mute
        duration_seconds: Duration to mute in seconds. 0 means indefinite.

    Returns:
        A dictionary with success status and message
    """
    success, message = whatsapp_mute_chat(chat_jid, duration_seconds)
    return {"success": success, "message": message}


@mcp.tool()
def unmute_chat(chat_jid: str) -> Dict[str, Any]:
    """Unmute a WhatsApp chat.

    Args:
        chat_jid: The JID of the chat to unmute

    Returns:
        A dictionary with success status and message
    """
    success, message = whatsapp_unmute_chat(chat_jid)
    return {"success": success, "message": message}


@mcp.tool()
def delete_chat(chat_jid: str) -> Dict[str, Any]:
    """Delete a WhatsApp chat.

    Args:
        chat_jid: The JID of the chat to delete

    Returns:
        A dictionary with success status and message
    """
    success, message = whatsapp_delete_chat(chat_jid)
    return {"success": success, "message": message}


@mcp.tool()
def star_message(chat_jid: str, message_id: str) -> Dict[str, Any]:
    """Star a WhatsApp message.

    Args:
        chat_jid: The JID of the chat containing the message
        message_id: The ID of the message to star

    Returns:
        A dictionary with success status and message
    """
    success, message = whatsapp_star_message(chat_jid, message_id, star=True)
    return {"success": success, "message": message}


@mcp.tool()
def unstar_message(chat_jid: str, message_id: str) -> Dict[str, Any]:
    """Unstar a WhatsApp message.

    Args:
        chat_jid: The JID of the chat containing the message
        message_id: The ID of the message to unstar

    Returns:
        A dictionary with success status and message
    """
    success, message = whatsapp_unstar_message(chat_jid, message_id)
    return {"success": success, "message": message}


@mcp.tool()
def forward_message(from_chat_jid: str, to_chat_jid: str, message_id: str) -> Dict[str, Any]:
    """Forward a WhatsApp message to another chat.

    Args:
        from_chat_jid: The JID of the chat containing the original message
        to_chat_jid: The JID of the chat to forward the message to
        message_id: The ID of the message to forward

    Returns:
        A dictionary with success status and message
    """
    success, message = whatsapp_forward_message(from_chat_jid, to_chat_jid, message_id)
    return {"success": success, "message": message}


@mcp.tool()
def label_chat(chat_jid: str, label_id: str, labeled: bool = True) -> Dict[str, Any]:
    """Label or unlabel a WhatsApp chat.

    Args:
        chat_jid: The JID of the chat to label
        label_id: The ID of the label to apply
        labeled: True to add the label, False to remove it

    Returns:
        A dictionary with success status and message
    """
    success, message = whatsapp_label_chat(chat_jid, label_id, labeled)
    return {"success": success, "message": message}


if __name__ == "__main__":
    # Initialize and run the server
    mcp.run(transport='stdio')
