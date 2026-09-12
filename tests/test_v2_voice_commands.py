from sthira_v2.voice_commands import MapIntent, parse_map_command


def test_voice_map_command_is_allowlisted():
    assert parse_map_command("show the alert area", confidence=0.95).intent is MapIntent.SHOW_ALERT_AREA
    assert parse_map_command("zoom in", confidence=0.95).intent is MapIntent.ZOOM_IN
    focused = parse_map_command("focus on Ward 8 School", confidence=0.95)
    assert focused.place == "Synthetic Ward 8 School"
    assert focused.target_id == "SZ-DEMO-01"
    assert parse_map_command("show my route", confidence=0.95).intent is MapIntent.SHOW_ROUTE
    assert parse_map_command("show my location", confidence=0.95).intent is MapIntent.SHOW_MY_LOCATION
    assert parse_map_command("change language to Malayalam", confidence=0.95).language == "ml-IN"
    assert parse_map_command("call emergency services", confidence=0.95).needs_confirmation


def test_low_confidence_and_prohibited_commands_have_no_side_effect():
    assert parse_map_command("show the alert area", confidence=0.4) is None
    assert parse_map_command("call 112", confidence=0.99).needs_confirmation
    assert parse_map_command("confirm arrival", confidence=0.99) is None
    assert parse_map_command("change capacity", confidence=0.99) is None
    assert parse_map_command("find the nearest safe shelter", confidence=0.99) is None
    assert parse_map_command("focus on an unknown school", confidence=0.99) is None
