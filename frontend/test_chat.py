from playwright.sync_api import sync_playwright

def test_chat_input():
    """Test if chat input works correctly"""

    with sync_playwright() as p:
        browser = p.chromium.launch(headless=True)
        page = browser.new_page()

        # Navigate to frontend
        page.goto('http://localhost:5173')

        # Wait for network idle
        page.wait_for_load_state('networkidle')

        # Take initial screenshot
        page.screenshot(path='/tmp/go-agent-initial.png', full_page=True)
        print("✓ Page loaded")

        # Check if textarea exists
        textarea = page.locator('textarea').first
        if not textarea.is_visible():
            print("✗ Textarea not visible")
            browser.close()
            return False

        print("✓ Textarea visible")

        # Type in textarea
        test_text = "列出文件"
        textarea.fill(test_text)

        # Check if value is set
        value = textarea.input_value()
        if value != test_text:
            print(f"✗ Value mismatch: got '{value}', expected '{test_text}'")
            browser.close()
            return False

        print(f"✓ Textarea value set to: {value}")

        # Check console logs
        console_messages = []
        def handle_console(msg):
            console_messages.append(msg.text)

        page.on('console', handle_console)

        # Try to send message (Enter key)
        textarea.press('Enter')

        # Wait a bit
        page.wait_for_timeout(2000)

        # Check console for input logs
        input_logs = [msg for msg in console_messages if 'onChange' in msg or 'onInput' in msg]
        if input_logs:
            print(f"✓ Console logs detected: {input_logs}")
        else:
            print("✗ No onChange/onInput logs in console")

        # Check if message was sent (check for new messages or loading state)
        # Look for any indication of sending
        has_loading = page.locator('text=Press ESC to cancel').is_visible()
        if has_loading:
            print("✓ Message sending detected (ESC hint visible)")
        else:
            print("⚠ No clear indication of message sending")

        browser.close()
        return True

if __name__ == '__main__':
    test_chat_input()
