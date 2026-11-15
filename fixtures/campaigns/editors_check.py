"""Checks character editors, DM gifts and transfers through a running go_dnd, as the same four users
as members_check.py: ttrpg_dm runs a new campaign, ttrpg_user2 plays in it, ttrpg_user is asked to
help edit, and ttrpg_user3 is an outsider.

    TTRPG_PASSWORD=... python3 editors_check.py
    DND_API=http://localhost:8004/api/v1 (the default)

It creates the campaign "Editors check <time>" and one character, and leaves them in place (the
character ends up owned by ttrpg_dm).
"""

import sys
import time

import members_check as m

User, status, expect = m.User, m.status, m.expect


def detail(user, cid):
    return user.call('GET', f'/characters/{cid}')[1]


def ac(user, cid):
    return user.call('GET', f'/characters/{cid}/sheet')[1]['armor_class']['value']


def main():
    dm, p2, p1, out = User('ttrpg_dm'), User('ttrpg_user2'), User('ttrpg_user'), User('ttrpg_user3')
    camp = status(dm, 'POST', '/campaigns', 200, 'creates a campaign', {'name': f'Editors check {time.strftime("%H:%M:%S")}'})
    gid = camp['id']
    token = status(dm, 'POST', f'/campaigns/{gid}/invites', 200, 'makes a link', {})['token']
    status(p2, 'POST', f'/invite-links/{token}/accept', 200, 'joins')
    ch = status(p2, 'POST', '/characters', 200, 'creates a character in the campaign', {'name': 'Editors check: p2', 'campaign_id': gid})
    cid = ch['id']

    print('editors')
    expect('no editors to start', status(p2, 'GET', f'/characters/{cid}/editors', 200, 'lists editors')['entities'], [])
    status(out, 'POST', f'/characters/{cid}/editors', 404, 'adds an editor', {'username': 'ttrpg_user3'})
    status(dm, 'POST', f'/characters/{cid}/editors', 403, 'adds an editor (not the owner)', {'username': 'ttrpg_dm'})
    status(p2, 'POST', f'/characters/{cid}/editors', 422, 'adds themselves (the owner)', {'username': 'ttrpg_user2'})
    status(p2, 'POST', f'/characters/{cid}/editors', 404, 'adds a user who never signed in', {'username': 'nobody_here_xyz'})
    eds = status(p2, 'POST', f'/characters/{cid}/editors', 200, 'adds ttrpg_user', {'username': 'ttrpg_user'})['entities']
    expect('the editor', [e['username'] for e in eds], ['ttrpg_user'])
    status(p1, 'GET', f'/characters/{cid}/sheet', 200, 'reads the sheet as an editor')
    status(p1, 'PATCH', f'/characters/{cid}', 200, 'edits as an editor', {'details': {'alignment': 'Lawful Good'}})
    expect('the character is in their list', cid in [c['id'] for c in p1.call('GET', '/characters/query?limit=500')[1]['entities']], True)
    status(p1, 'DELETE', f'/characters/{cid}', 403, 'deletes it as an editor')
    status(p1, 'POST', f'/characters/{cid}/editors', 403, 'adds an editor as an editor', {'username': 'ttrpg_user3'})
    status(p1, 'PUT', f'/characters/{cid}/transfer', 403, 'hands it over as an editor', {'username': 'ttrpg_user3'})
    left = status(p1, 'DELETE', f'/characters/{cid}/editors/{eds[0]["user_id"]}', 200, 'takes themselves off')
    expect('their list after', left['entities'], [])
    status(p1, 'PATCH', f'/characters/{cid}', 404, 'edits after leaving', {'details': {}})

    print('gifts')
    base_ac = ac(p2, cid)
    gift = {'items': [{'custom_name': 'Sending Stone', 'custom_description': 'Glows when the other one speaks.'}],
            'currency': {'gp': 10}, 'modifiers': [{'label': "DM's blessing", 'target': 'ac', 'op': 'add', 'value': 1}]}
    gold = detail(p2, cid)['currency'].get('gp', 0)
    status(out, 'POST', f'/campaigns/{gid}/characters/{cid}/gifts', 404, 'gives', gift)
    status(p2, 'POST', f'/campaigns/{gid}/characters/{cid}/gifts', 403, 'gives (a player)', gift)
    after = status(dm, 'POST', f'/campaigns/{gid}/characters/{cid}/gifts', 200, 'gives a stone, 10 gp and +1 AC', gift)
    stone = [i for i in after['items'] if i['custom_name'] == 'Sending Stone'][0]
    blessing = [x for x in after['modifiers'] if x['label'] == "DM's blessing"][0]
    expect('the stone is marked a gift', (stone['given_by'] is not None, stone['given_campaign_id']), (True, gid))
    expect('gold added', after['currency']['gp'], gold + 10)
    expect('the blessing is the campaign\'s', (blessing['added_as'], blessing['campaign_id']), ('dm', gid))
    expect('AC went up by 1', ac(p2, cid), base_ac + 1)
    status(dm, 'POST', f'/campaigns/{gid}/characters/{cid}/gifts', 422, 'gives negative gold', {'currency': {'gp': -5}})
    status(dm, 'POST', f'/campaigns/{gid}/characters/{cid}/gifts', 422, 'gives nothing', {})
    status(p2, 'PATCH', f'/characters/{cid}/items/{stone["id"]}', 403, 'renames the DM\'s stone', {'custom_name': 'Rock'})
    status(dm, 'PATCH', f'/characters/{cid}/items/{stone["id"]}', 403, 'equips the stone (the player\'s call)', {'equipped': True})
    status(dm, 'PATCH', f'/characters/{cid}/items/{stone["id"]}', 200, 'renames their stone', {'custom_name': 'Sending Stone (A)'})
    status(p2, 'PATCH', f'/characters/{cid}/items/{stone["id"]}', 200, 'equips the stone', {'equipped': True})
    status(dm, 'DELETE', f'/characters/{cid}/items/{stone["id"]}', 409, 'takes back the changed stone')
    status(p2, 'PATCH', f'/characters/{cid}/modifiers/{blessing["id"]}', 403, 'switches off the DM\'s blessing', {'active': False})
    status(p2, 'DELETE', f'/characters/{cid}/modifiers/{blessing["id"]}', 403, 'removes the DM\'s blessing')
    status(p2, 'POST', f'/characters/{cid}/modifiers', 422, 'adds a boon as the DM',
           {'label': 'fake', 'target': 'ac', 'op': 'add', 'value': 5, 'added_as': 'dm'})
    extra = status(dm, 'POST', f'/campaigns/{gid}/characters/{cid}/gifts', 200, 'gives a potion', {'items': [{'custom_name': 'Odd potion'}]})
    potion = [i for i in extra['items'] if i['custom_name'] == 'Odd potion'][0]
    status(dm, 'DELETE', f'/characters/{cid}/items/{potion["id"]}', 200, 'takes back the untouched potion')

    print('a DM boon counts only in its campaign')
    status(p2, 'DELETE', f'/campaigns/{gid}/characters/{cid}', 200, 'takes the character out')
    expect('AC without the blessing', ac(p2, cid), base_ac)
    expect('the blessing is kept', [x['id'] for x in detail(p2, cid)['modifiers'] if x['label'] == "DM's blessing"], [blessing['id']])
    status(dm, 'POST', f'/campaigns/{gid}/characters/{cid}/gifts', 404, 'gives to a character that left', {'currency': {'gp': 1}})
    status(p2, 'POST', f'/campaigns/{gid}/characters/{cid}', 200, 'brings it back')
    expect('AC with the blessing again', ac(p2, cid), base_ac + 1)
    status(dm, 'PATCH', f'/characters/{cid}/modifiers/{blessing["id"]}', 200, 'switches the blessing off', {'active': False})
    expect('AC with it off', ac(p2, cid), base_ac)

    print('transfers')
    status(p2, 'PUT', f'/characters/{cid}/transfer', 422, 'offers it to someone outside the campaign', {'username': 'ttrpg_user3'})
    status(dm, 'PUT', f'/characters/{cid}/transfer', 403, 'offers someone else\'s character', {'username': 'ttrpg_dm'})
    offer = status(p2, 'PUT', f'/characters/{cid}/transfer', 200, 'offers it to the DM', {'username': 'ttrpg_dm'})
    expect('the offer', offer['transfer']['to_username'], 'ttrpg_dm')
    status(p2, 'POST', f'/characters/{cid}/editors', 200, 'adds an editor before handing over', {'username': 'ttrpg_user'})
    incoming = status(dm, 'GET', '/character-transfers', 200, 'lists offers')['entities']
    expect('the DM sees it', [t['character_name'] for t in incoming if t['character_id'] == cid], ['Editors check: p2'])
    status(out, 'POST', f'/character-transfers/{cid}/accept', 404, 'accepts someone else\'s offer')
    took = status(dm, 'POST', f'/character-transfers/{cid}/accept', 200, 'accepts')
    expect('the DM owns it', took['owner_id'], camp['owner_id'])
    expect('still in the campaign', took['campaign_id'], gid)
    expect('editors cleared', dm.call('GET', f'/characters/{cid}/editors')[1]['entities'], [])
    status(p2, 'GET', f'/characters/{cid}/sheet', 403, 'reads the sheet they gave away (party summary)')
    status(p2, 'PATCH', f'/characters/{cid}', 403, 'edits the character they gave away', {'details': {}})
    status(dm, 'PUT', f'/characters/{cid}/transfer', 200, 'offers it back for an hour', {'username': 'ttrpg_user2', 'expires_in': '1h'})
    status(p2, 'POST', f'/character-transfers/{cid}/decline', 200, 'declines')
    expect('no offer left', dm.call('GET', f'/characters/{cid}/transfer')[1]['transfer'], None)

    print('ALL AS EXPECTED' if not m.failures else f'{m.failures} unexpected')
    return 1 if m.failures else 0


if __name__ == '__main__':
    sys.exit(main())
