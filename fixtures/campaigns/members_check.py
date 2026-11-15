"""Checks campaign members, invites and access through a running go_dnd, as four signed-in users:
ttrpg_dm runs a new campaign, ttrpg_user2 joins by link, ttrpg_user joins by a username invite, and
ttrpg_user3 is an outsider. Each step says what it expects (status, or a value) and the run ends
with the count that didn't match.

    TTRPG_PASSWORD=... python3 members_check.py          (every test user shares the password)
    DND_API=http://localhost:8004/api/v1 (the default)

It creates the campaign "Members check <time>" and two characters, and leaves them in place.
"""

import http.cookiejar
import json
import os
import sys
import time
import urllib.error
import urllib.parse
import urllib.request

BASE = os.environ.get('DND_API', 'http://localhost:8004/api/v1')
PASSWORD = os.environ['TTRPG_PASSWORD']
failures = 0


class User:
    def __init__(self, name):
        self.name = name
        self.opener = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))
        form = urllib.parse.urlencode({'username': name, 'password': PASSWORD}).encode()
        self.opener.open(urllib.request.Request(f'{BASE}/auth/session/token', data=form, method='POST',
                                                headers={'Content-Type': 'application/x-www-form-urlencoded'}), timeout=20)

    def call(self, method, path, body=None):
        """(status, parsed body)"""
        data = json.dumps(body).encode() if body is not None else None
        req = urllib.request.Request(f'{BASE}{path}', data=data, method=method,
                                     headers={'Content-Type': 'application/json'} if data else {})
        try:
            with self.opener.open(req, timeout=20) as res:
                return res.status, json.loads(res.read() or b'null')
        except urllib.error.HTTPError as e:
            raw = e.read()
            try:
                return e.code, json.loads(raw)
            except ValueError:
                return e.code, raw.decode()[:200]


def expect(label, got, want):
    global failures
    ok = got == want
    failures += 0 if ok else 1
    print(f'  {"ok  " if ok else "FAIL"} {label}: {got}{"" if ok else f" (want {want})"}')


def status(user, method, path, want, label, body=None):
    code, out = user.call(method, path, body)
    expect(f'{user.name} {label}', code, want)
    return out


def main():
    dm, p2, p1, out = User('ttrpg_dm'), User('ttrpg_user2'), User('ttrpg_user'), User('ttrpg_user3')
    print('campaign and link invite')
    c = status(dm, 'POST', '/campaigns', 200, 'creates a campaign', {'name': f'Members check {time.strftime("%H:%M:%S")}'})
    cid = c['id']
    expect('creator\'s role', c['role'], 'dm')
    expect('default access settings', c['settings']['access'], {'invite': 'dms', 'party_sheets': 'summary'})
    status(out, 'GET', f'/campaigns/{cid}', 404, 'sees the campaign')
    link = status(dm, 'POST', f'/campaigns/{cid}/invites', 200, 'makes a link', {'expires_in': '1h'})
    token = link['token']
    preview = status(p2, 'GET', f'/invite-links/{token}', 200, 'previews the link')
    expect('preview names the campaign', preview['campaign_name'], c['name'])
    joined = status(p2, 'POST', f'/invite-links/{token}/accept', 200, 'joins by link')
    expect('joiner\'s role', joined['role'], 'player')
    status(p2, 'POST', f'/invite-links/{token}/accept', 200, 'joins again (no change)')
    invites = status(dm, 'GET', f'/campaigns/{cid}/invites', 200, 'lists invites')['entities']
    expect('the link counted one use', [i['uses'] for i in invites if i['kind'] == 'link'], [1])

    print('who may do what in the campaign')
    members = status(p2, 'GET', f'/campaigns/{cid}/members', 200, 'lists members')['entities']
    expect('members, DMs first', [(m['username'], m['role'], m['owner']) for m in members],
           [('ttrpg_dm', 'dm', True), ('ttrpg_user2', 'player', False)])
    status(out, 'GET', f'/campaigns/{cid}/members', 404, 'lists members')
    status(p2, 'PATCH', f'/campaigns/{cid}', 403, 'changes the campaign', {'description': 'mine now'})
    status(p2, 'POST', f'/campaigns/{cid}/invites', 403, 'makes a link (invite: dms)', {})
    status(p2, 'GET', f'/campaigns/{cid}/invites', 403, 'lists invites')
    status(p2, 'DELETE', f'/campaigns/{cid}', 403, 'deletes the campaign')
    status(out, 'DELETE', f'/campaigns/{cid}', 404, 'deletes the campaign')
    patched = status(dm, 'PATCH', f'/campaigns/{cid}', 200, 'lets members invite', {'settings': {'access': {'invite': 'members'}}})
    expect('access merged', patched['settings']['access'], {'invite': 'members', 'party_sheets': 'summary'})
    status(dm, 'PATCH', f'/campaigns/{cid}', 422, 'sets a bad access value', {'settings': {'access': {'invite': 'anyone'}}})
    status(p2, 'POST', f'/campaigns/{cid}/invites', 200, 'makes a link (invite: members)', {'expires_in': '1h'})

    print('invite by username')
    status(dm, 'POST', f'/campaigns/{cid}/invites', 404, 'invites a user who never signed in', {'username': 'nobody_here_xyz'})
    status(dm, 'POST', f'/campaigns/{cid}/invites', 409, 'invites a member', {'username': 'ttrpg_user2'})
    status(dm, 'POST', f'/campaigns/{cid}/invites', 200, 'invites ttrpg_user', {'username': 'ttrpg_user'})
    status(dm, 'POST', f'/campaigns/{cid}/invites', 409, 'invites ttrpg_user again', {'username': 'ttrpg_user'})
    mine = status(p1, 'GET', '/invites', 200, 'lists their invites')['entities']
    pending = [i for i in mine if i['campaign_id'] == cid]
    expect('the invite is there', len(pending), 1)
    status(out, 'POST', f'/invites/{pending[0]["id"]}/accept', 404, 'accepts someone else\'s invite')
    status(p1, 'POST', f'/invites/{pending[0]["id"]}/accept', 200, 'accepts')
    expect('accepted invite is gone', [i for i in p1.call('GET', '/invites')[1]['entities'] if i['campaign_id'] == cid], [])

    print('characters: owner, DM, party, outsider')
    ch = status(p2, 'POST', '/characters', 200, 'creates a character in the campaign', {'name': 'Members check: p2', 'campaign_id': cid})
    chid = ch['id']
    status(out, 'POST', '/characters', 404, 'creates a character in the campaign', {'name': 'x', 'campaign_id': cid})
    status(dm, 'GET', f'/characters/{chid}/sheet', 200, 'reads the player\'s sheet')
    status(dm, 'PATCH', f'/characters/{chid}', 403, 'edits the player\'s character', {'name': 'DM was here'})
    status(p1, 'GET', f'/characters/{chid}/sheet', 403, 'reads a party member\'s sheet (summary)')
    party = status(p1, 'GET', f'/campaigns/{cid}/characters', 200, 'reads the party list')['entities']
    expect('party list names the player', [(e['name'], e['owner_username']) for e in party], [('Members check: p2', 'ttrpg_user2')])
    status(dm, 'PATCH', f'/campaigns/{cid}', 200, 'shows full party sheets', {'settings': {'access': {'party_sheets': 'full'}}})
    status(p1, 'GET', f'/characters/{chid}/sheet', 200, 'reads a party member\'s sheet (full)')
    status(p1, 'PATCH', f'/characters/{chid}', 403, 'edits a party member\'s character', {'name': 'mine'})
    status(out, 'GET', f'/characters/{chid}', 404, 'reads the character')
    status(out, 'GET', f'/characters/{chid}/sheet', 404, 'reads the sheet')
    status(p2, 'PATCH', f'/characters/{chid}', 200, 'edits their own character', {'details': {'alignment': 'Neutral'}})

    print('roles')
    status(p2, 'PATCH', f'/campaigns/{cid}/members/{members[0]["user_id"]}', 403, 'demotes the owner', {'role': 'player'})
    status(dm, 'PATCH', f'/campaigns/{cid}/members/{members[0]["user_id"]}', 403, 'demotes themselves (the owner)', {'role': 'player'})
    p2id = members[1]['user_id']
    status(dm, 'PATCH', f'/campaigns/{cid}/members/{p2id}', 200, 'promotes ttrpg_user2', {'role': 'dm'})
    status(p2, 'PATCH', f'/campaigns/{cid}', 200, 'changes the campaign as a DM', {'description': 'co-DM'})
    status(p2, 'DELETE', f'/campaigns/{cid}', 403, 'deletes the campaign as a DM')
    status(dm, 'PATCH', f'/campaigns/{cid}/members/{p2id}', 200, 'demotes ttrpg_user2', {'role': 'player'})
    status(dm, 'DELETE', f'/campaigns/{cid}/members/{members[0]["user_id"]}', 403, 'leaves (the owner)')

    print('leaving and removing')
    c1 = status(p1, 'POST', '/characters', 200, 'creates a character in the campaign', {'name': 'Members check: p1', 'campaign_id': cid})
    p1id = c1['owner_id']
    status(p2, 'DELETE', f'/campaigns/{cid}/members/{p1id}', 403, 'removes another player')
    status(dm, 'DELETE', f'/campaigns/{cid}/members/{p1id}', 200, 'removes ttrpg_user')
    expect('their character left with them', p1.call('GET', f'/characters/{c1["id"]}')[1].get('campaign_id'), None)
    status(p1, 'GET', f'/campaigns/{cid}', 404, 'sees the campaign after removal')
    status(p2, 'DELETE', f'/campaigns/{cid}/members/{p2id}', 200, 'leaves')
    expect('their character left with them', p2.call('GET', f'/characters/{chid}')[1].get('campaign_id'), None)
    status(dm, 'GET', f'/characters/{chid}', 404, 'reads the character that left')

    print('revoked links')
    link2 = status(dm, 'POST', f'/campaigns/{cid}/invites', 200, 'makes a link', {'expires_in': '1d', 'max_uses': 1})
    status(dm, 'DELETE', f'/campaigns/{cid}/invites/{link2["invite"]["id"]}', 200, 'revokes it')
    status(out, 'POST', f'/invite-links/{link2["token"]}/accept', 410, 'joins by a revoked link')
    status(out, 'GET', '/invite-links/not-a-real-token-1234567890', 404, 'previews a made-up link')

    print('ALL AS EXPECTED' if not failures else f'{failures} unexpected')
    return 1 if failures else 0


if __name__ == '__main__':
    sys.exit(main())
