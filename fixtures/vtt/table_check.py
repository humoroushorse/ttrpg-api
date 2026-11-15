"""Checks the tabletop through two running go_dnd instances (two pods behind no load balancer), as four
signed-in users: ttrpg_dm runs a new campaign on pod A, ttrpg_user2 plays on pod B, ttrpg_user plays on
pod A, and ttrpg_user3 is an outsider on pod B. The DM and both players keep an event stream open, so
each step checks what every one of them was sent (and wasn't), not just the HTTP answers.

    TTRPG_PASSWORD=... python3 table_check.py            (every test user shares the password)
    DND_API_A=http://localhost:8014/api/v1  DND_API_B=http://localhost:8015/api/v1   (the defaults)

Both instances need the same NATS and database. It creates the campaign "Table check <time>", three
characters and some scenes, and deletes the campaign at the end.
"""

import http.cookiejar
import json
import os
import queue
import sys
import threading
import time
import urllib.error
import urllib.parse
import urllib.request
import uuid

POD_A = os.environ.get('DND_API_A', 'http://localhost:8014/api/v1')
POD_B = os.environ.get('DND_API_B', 'http://localhost:8015/api/v1')
PASSWORD = os.environ['TTRPG_PASSWORD']
failures = 0


class User:
    def __init__(self, name, base):
        self.name, self.base = name, base
        self.opener = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))
        form = urllib.parse.urlencode({'username': name, 'password': PASSWORD}).encode()
        self.opener.open(urllib.request.Request(f'{base}/auth/session/token', data=form, method='POST',
                                                headers={'Content-Type': 'application/x-www-form-urlencoded'}), timeout=20)

    def call(self, method, path, body=None):
        """(status, parsed body)"""
        data = json.dumps(body).encode() if body is not None else None
        req = urllib.request.Request(f'{self.base}{path}', data=data, method=method,
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


class Stream:
    """An open event stream, read in a thread. Every event it got stays in .seen."""

    opened = []  # every stream, for the check that no player was ever sent a secret

    def __init__(self, user, cid, after=None, last_event_id=None):
        Stream.opened.append(self)
        self.user, self.q, self.seen, self.pending, self.text = user, queue.Queue(), [], [], []
        url = f'{user.base}/vtt/campaigns/{cid}/events' + ('' if after is None else f'?after={after}')
        headers = {'Accept': 'text/event-stream'}
        if last_event_id is not None:
            headers['Last-Event-ID'] = str(last_event_id)
        self.res = user.opener.open(urllib.request.Request(url, headers=headers), timeout=60)
        self.headers = dict(self.res.headers)
        self.closed = threading.Event()
        threading.Thread(target=self._read, daemon=True).start()

    def _read(self):
        frame = {}
        try:
            for raw in self.res:
                line = raw.decode().rstrip('\r\n')
                self.text.append(line)
                if line == '':
                    if 'event' in frame:
                        frame['data'] = json.loads(frame.get('data', 'null'))
                        self.q.put(frame)
                    frame = {}
                elif not line.startswith(':'):
                    key, _, value = line.partition(':')
                    frame[key] = value[1:] if value.startswith(' ') else value
        except Exception:  # closed by us, or the server ended it
            pass
        self.closed.set()
        self.q.put(None)

    def _pull(self, timeout):
        try:
            e = self.q.get(timeout=timeout)
        except queue.Empty:
            return False
        if e is not None:
            self.seen.append(e)
            self.pending.append(e)
        return e is not None

    def next(self, match, timeout=5):
        """The first event (not taken yet) that matches, waiting up to timeout."""
        deadline = time.time() + timeout
        while True:
            for i, e in enumerate(self.pending):
                if match(e):
                    return self.pending.pop(i)
            left = deadline - time.time()
            if left <= 0 or (not self._pull(left) and self.closed.is_set() and self.q.empty()):
                return None

    def none(self, match, wait=0.6):
        """True when nothing matching arrives within wait."""
        return self.next(match, wait) is None

    def ended(self, timeout=5):
        return self.closed.wait(timeout)

    def close(self):
        try:
            self.res.close()
        except Exception:
            pass

    def everything(self):
        return '\n'.join(self.text)


def typed(t, **want):
    def match(e):
        if e.get('event') != t:
            return False
        payload = (e['data'] or {}).get('payload') or {}
        token = payload.get('token') or {}
        for key, value in want.items():
            have = payload.get(key, token.get(key, (e['data'] or {}).get(key)))
            if have != value:
                return False
        return True
    return match


def expect(label, got, want):
    global failures
    ok = got == want
    failures += 0 if ok else 1
    print(f'  {"ok  " if ok else "FAIL"} {label}: {got}{"" if ok else f" (want {want})"}')


def status(user, method, path, want, label, body=None):
    code, out = user.call(method, path, body)
    expect(f'{user.name} {label}', code, want)
    if code != want and isinstance(out, dict):
        print(f'        {out.get("detail")}')
    return out


def got(stream, label, match, timeout=5):
    e = stream.next(match, timeout)
    expect(f'{stream.user.name} is sent {label}', e is not None, True)
    return e


def not_got(stream, label, match, wait=0.6):
    expect(f'{stream.user.name} isn\'t sent {label}', stream.none(match, wait), True)


def main():
    dm, p2 = User('ttrpg_dm', POD_A), User('ttrpg_user2', POD_B)
    p1, out = User('ttrpg_user', POD_A), User('ttrpg_user3', POD_B)
    print(f'pods: A {POD_A} (ttrpg_dm, ttrpg_user), B {POD_B} (ttrpg_user2, ttrpg_user3)')

    print('a campaign, two players, their characters')
    c = status(dm, 'POST', '/campaigns', 200, 'creates a campaign', {'name': f'Table check {time.strftime("%H:%M:%S")}'})
    cid, vtt = c['id'], f'/vtt/campaigns/{c["id"]}'
    token = status(dm, 'POST', f'/campaigns/{cid}/invites', 200, 'makes a link', {'expires_in': '1h'})['token']
    status(p2, 'POST', f'/invite-links/{token}/accept', 200, 'joins')
    status(p1, 'POST', f'/invite-links/{token}/accept', 200, 'joins')
    ezmerelda = status(p2, 'POST', '/characters', 200, 'creates Ezmerelda', {'name': 'Table check: Ezmerelda', 'campaign_id': cid})['id']
    ismark = status(p1, 'POST', '/characters', 200, 'creates Ismark', {'name': 'Table check: Ismark', 'campaign_id': cid})['id']
    stranger = status(out, 'POST', '/characters', 200, 'creates a character elsewhere', {'name': 'Table check: stranger'})['id']
    members = {m['username']: m['user_id'] for m in dm.call('GET', f'/campaigns/{cid}/members')[1]['entities']}

    print('before there is a table')
    status(out, 'GET', f'{vtt}/scenes', 404, 'lists the scenes (not in the campaign)')
    st = status(p2, 'GET', f'{vtt}/state', 200, 'reads the state')
    expect('no scene yet', (st.get('scene'), st.get('seq')), (None, 0))
    status(p2, 'POST', f'{vtt}/scenes', 403, 'creates a scene', {'name': 'x', 'width_cells': 5, 'height_cells': 5})
    status(dm, 'POST', f'{vtt}/scenes', 422, 'creates a scene with no size', {'name': 'x'})

    print('a scene tree: Barovia > Village > Tavern, Barovia > Crypt')
    barovia = status(dm, 'POST', f'{vtt}/scenes', 200, 'creates Barovia (overland, gridless, miles)', {
        'name': 'Barovia', 'grid_type': 'none', 'width_cells': 40, 'height_cells': 30, 'cell_px': 100,
        'grid_distance': 1, 'grid_units': 'mi', 'background_sources': [
            {'url': 'https://example.test/barovia.avif', 'type': 'image/avif'}, {'url': 'https://example.test/barovia.png'}]})
    village = status(dm, 'POST', f'{vtt}/scenes', 200, 'creates the village', {'name': 'Village of Barovia', 'parent_id': barovia['id'], 'width_cells': 50, 'height_cells': 50})
    tavern = status(dm, 'POST', f'{vtt}/scenes', 200, 'creates the tavern', {'name': 'Blood on the Vine', 'parent_id': village['id'], 'width_cells': 20, 'height_cells': 15, 'cell_px': 70.5})
    crypt = status(dm, 'POST', f'{vtt}/scenes', 200, 'creates the crypt', {'name': 'Crypt', 'parent_id': barovia['id'], 'width_cells': 20, 'height_cells': 20})
    expect('defaults', (tavern['grid_type'], tavern['grid_distance'], tavern['grid_units'], tavern['settings']['token_reveal']), ('square', 5, 'ft', 'numbers'))
    status(dm, 'PATCH', f'/vtt/scenes/{village["id"]}', 422, 'moves the village under its own tavern', {'parent_id': tavern['id']})
    status(dm, 'PATCH', f'/vtt/scenes/{tavern["id"]}', 422, 'sets a javascript: background', {'background_sources': [{'url': 'javascript:alert(1)'}]})
    status(out, 'PATCH', f'/vtt/scenes/{tavern["id"]}', 404, 'renames a scene (not in the campaign)', {'name': 'mine'})
    scenes = status(dm, 'GET', f'{vtt}/scenes', 200, 'lists the scenes')
    expect('four scenes, none active', (len(scenes['entities']), scenes['active_scene_id']), (4, None))

    print('streams: one per user, two pods')
    s_dm = Stream(dm, cid, after=dm.call('GET', f'{vtt}/state')[1]['seq'])
    s_p2 = Stream(p2, cid, after=p2.call('GET', f'{vtt}/state')[1]['seq'])
    s_p1 = Stream(p1, cid, after=p1.call('GET', f'{vtt}/state')[1]['seq'])
    expect('stream headers', (s_dm.headers.get('Content-Type'), s_dm.headers.get('Cache-Control'), s_dm.headers.get('X-Accel-Buffering')),
           ('text/event-stream', 'no-cache, no-transform', 'no'))
    status(out, 'GET', f'{vtt}/events?after=0', 404, 'opens the stream (not in the campaign)')
    status(p2, 'GET', f'{vtt}/events', 422, 'opens the stream without a seq')

    print('the DM opens the tavern')
    status(dm, 'PUT', f'{vtt}/active-scene', 200, 'activates the tavern', {'scene_id': tavern['id']})
    for s in (s_dm, s_p2, s_p1):
        got(s, 'scene.activated', typed('scene.activated', scene_id=tavern['id']))
    expect('a player\'s copy has no DM details', 'previous_scene_id' in s_p2.everything(), False)
    listed = status(p2, 'GET', f'{vtt}/scenes', 200, 'lists the scenes')
    expect('a player sees only the tavern, outside the tree', [(e['name'], e['parent_id']) for e in listed['entities']], [('Blood on the Vine', None)])
    expect('the scene is in the player\'s state', p2.call('GET', f'{vtt}/state')[1]['scene']['name'], 'Blood on the Vine')

    print('tokens')
    sid = tavern['id']

    def command(user, want, label, **body):
        return status(user, 'POST', f'{vtt}/commands', want, label, {'scene_id': body.pop('scene_id', sid), **body})

    t2 = command(dm, 200, 'places Ezmerelda', type='token.add', kind='character', character_id=ezmerelda, x=2, y=2)['token']
    t1 = command(dm, 200, 'places Ismark', type='token.add', kind='character', character_id=ismark, x=3, y=2)['token']
    spawn = command(dm, 200, 'places a hostile marker', type='token.add', kind='marker', name='Vampire Spawn', disposition='hostile', x=10, y=5)['token']
    spawn2 = command(dm, 200, 'places another', type='token.add', kind='marker', name='Vampire Spawn', disposition='hostile', x=11, y=5)['token']
    ambush = command(dm, 200, 'hides one on the DM layer', type='token.add', kind='marker', name='Ambush', layer='dm', disposition='hostile', x=12, y=5)['token']
    raven = command(dm, 200, 'places Ezmerelda\'s raven', type='token.add', kind='marker', name='Raven', controller_character_id=ezmerelda, disposition='party', size_cells=0.5, x=2.5, y=3)['token']
    expect('the sheet\'s name, party, #none', (t2['name'], t2['disposition'], t2['number']), ('Table check: Ezmerelda', 'party', None))
    expect('hostile markers are numbered, players see only "#n"', (spawn['number'], spawn['player_name'], spawn2['number']), (1, '', 2))
    command(p2, 403, 'places a token', type='token.add', kind='marker', name='mine', x=1, y=1)
    command(dm, 409, 'places Ezmerelda twice in a scene', type='token.add', kind='character', character_id=ezmerelda, x=5, y=5)
    command(dm, 422, 'places a character from outside the campaign', type='token.add', kind='character', character_id=stranger, x=5, y=5)
    command(dm, 422, 'places a token off the map', type='token.add', kind='marker', name='x', x=19.5, y=1)
    command(dm, 422, 'sends a field token.move doesn\'t take', type='token.move', token_id=spawn['id'], x=1, y=1, name='x')

    got(s_p2, 'the hostile marker as just #1', typed('token.added', id=spawn['id'], name='', number=1))
    got(s_p2, 'Ezmerelda', typed('token.added', id=t2['id'], can_move=True))
    got(s_p2, 'Ismark, not movable by them', typed('token.added', id=t1['id'], can_move=False))
    got(s_p2, 'the raven, movable by them', typed('token.added', id=raven['id'], can_move=True))
    not_got(s_p2, 'the DM-layer token', typed('token.added', id=ambush['id']))
    got(s_dm, 'the DM-layer token', typed('token.added', id=ambush['id'], name='Ambush'))
    got(s_dm, 'the true name', typed('token.added', id=spawn['id'], name='Vampire Spawn', player_name=''))

    print('moves, across pods')
    seq = command(p2, 200, 'moves Ezmerelda (on pod B)', type='token.move', token_id=t2['id'], x=4, y=4)['seq']
    got(s_dm, 'the move from pod B (on pod A)', typed('token.moved', token_id=t2['id'], x=4))
    got(s_p1, 'the move from pod B (on pod A)', typed('token.moved', token_id=t2['id'], x=4))
    got(s_p2, 'their own move back', lambda e: e.get('event') == 'token.moved' and e.get('id') == str(seq))
    command(p2, 200, 'moves the raven they control', type='token.move', token_id=raven['id'], x=3.5, y=3.5)
    command(p2, 403, 'moves Ismark', type='token.move', token_id=t1['id'], x=5, y=5)
    command(p2, 403, 'moves the vampire spawn', type='token.move', token_id=spawn['id'], x=5, y=5)
    command(p2, 404, 'moves the hidden token', type='token.move', token_id=ambush['id'], x=5, y=5)
    command(p1, 403, 'moves Ezmerelda', type='token.move', token_id=t2['id'], x=5, y=5)
    command(dm, 200, 'moves the vampire spawn', type='token.move', token_id=spawn['id'], x=9, y=5)
    retry = str(uuid.uuid4())
    first = command(p2, 200, 'moves with a command_id', type='token.move', token_id=t2['id'], x=6, y=6, command_id=retry)['seq']
    again = command(p2, 200, 'retries the same command', type='token.move', token_id=t2['id'], x=7, y=7, command_id=retry)['seq']
    where = [t for t in p2.call('GET', f'{vtt}/state')[1]['tokens'] if t['id'] == t2['id']][0]
    expect('a retry answers with the first seq and changes nothing', (again == first, where['x']), (True, 6))
    expect('players\' tokens in state, with can_move',
           sorted((t['name'], t['can_move']) for t in p2.call('GET', f'{vtt}/state')[1]['tokens']),
           sorted([('', False), ('', False), ('Raven', True), ('Table check: Ezmerelda', True), ('Table check: Ismark', False)]))

    print('pings')
    status(p2, 'POST', f'{vtt}/commands', 200, 'pings', {'type': 'ping', 'scene_id': sid, 'x': 5, 'y': 5})
    ping = got(s_dm, 'the ping', typed('ping', x=5))
    expect('a ping has no seq', ping is not None and 'id' not in ping, True)
    got(s_p1, 'the ping', typed('ping', x=5))
    codes = [p2.call('POST', f'{vtt}/commands', {'type': 'ping', 'scene_id': sid, 'x': 1, 'y': 1})[0] for _ in range(14)]
    expect('pinging nonstop is slowed down', 429 in codes, True)

    print('names and layers revealed and hidden')
    command(dm, 200, 'names the spawn for players', type='token.update', token_id=spawn['id'], player_name='Hooded figure', player_disposition='friendly')
    got(s_p2, 'the shown name and disposition', typed('token.updated', id=spawn['id'], name='Hooded figure', disposition='friendly'))
    command(dm, 200, 'reveals the ambush as "Shadow"', type='token.update', token_id=ambush['id'], layer='tokens', player_name='Shadow')
    got(s_p2, 'the revealed token', typed('token.added', id=ambush['id'], name='Shadow'))
    command(dm, 200, 'hides it again', type='token.update', token_id=ambush['id'], layer='dm')
    got(s_p2, 'it gone', typed('token.removed', token_id=ambush['id']))
    command(dm, 409, 'gives the second spawn the first one\'s number', type='token.update', token_id=spawn2['id'], number=1)

    print('reconnecting replays what was missed')
    last = max(int(e['id']) for e in s_p2.seen if 'id' in e)
    s_p2.close()
    command(dm, 200, 'moves the spawn while ttrpg_user2 is away', type='token.move', token_id=spawn['id'], x=8, y=5)
    command(dm, 200, 'and again', type='token.move', token_id=spawn['id'], x=7, y=5)
    s_p2 = Stream(p2, cid, after=0, last_event_id=last)
    got(s_p2, 'the first missed move', typed('token.moved', token_id=spawn['id'], x=8))
    got(s_p2, 'the second', typed('token.moved', token_id=spawn['id'], x=7))
    ahead = Stream(p2, cid, last_event_id=last + 1000)
    got(ahead, 'reload when asking for a seq the log doesn\'t have', typed('reload'))
    expect('that stream ends', ahead.ended(), True)

    print('split party: ttrpg_user goes to the crypt')
    status(dm, 'PUT', f'{vtt}/members/{members["ttrpg_user"]}/scene', 200, 'moves ttrpg_user to the crypt', {'scene_id': crypt['id']})
    got(s_p1, 'where they went', typed('member.moved', scene_id=crypt['id']))
    not_got(s_p2, 'where someone else went', typed('member.moved'))
    status(dm, 'PUT', f'{vtt}/members/{members["ttrpg_dm"]}/scene', 422, 'moves a DM', {'scene_id': crypt['id']})
    skeleton = command(dm, 200, 'places a skeleton in the crypt', type='token.add', scene_id=crypt['id'], kind='marker', name='Skeleton', disposition='hostile', x=1, y=1)['token']
    got(s_p1, 'the crypt\'s token', typed('token.added', id=skeleton['id'], name=''))
    not_got(s_p2, 'the crypt\'s token', typed('token.added', id=skeleton['id']))
    command(dm, 200, 'moves Ezmerelda in the tavern', type='token.move', token_id=t2['id'], x=9, y=9)
    not_got(s_p1, 'the tavern\'s move', typed('token.moved', token_id=t2['id'], x=9))
    expect('their state is the crypt', p1.call('GET', f'{vtt}/state')[1]['scene']['name'], 'Crypt')
    status(p1, 'GET', f'{vtt}/state?scene={sid}', 404, 'reads the tavern')
    command(p1, 404, 'moves Ismark in the tavern', type='token.move', token_id=t1['id'], x=5, y=5)
    status(dm, 'DELETE', f'/vtt/scenes/{crypt["id"]}', 409, 'trashes the crypt with a player on it')
    status(dm, 'DELETE', f'/vtt/scenes/{barovia["id"]}', 409, 'trashes Barovia (the active scene is in it)')
    expect('the DM sees who is where', dm.call('GET', f'{vtt}/state')[1]['members'], [{'user_id': members['ttrpg_user'], 'scene_id': crypt['id']}])
    status(dm, 'PUT', f'{vtt}/active-scene', 200, 'moves everyone to the village', {'scene_id': village['id']})
    got(s_p1, 'everyone together again', typed('scene.activated', scene_id=village['id']))
    expect('ttrpg_user is in the village', p1.call('GET', f'{vtt}/state')[1]['scene']['name'], 'Village of Barovia')
    expect('the crypt goes to the trash', status(dm, 'DELETE', f'/vtt/scenes/{crypt["id"]}', 200, 'trashes the crypt'), [crypt['id']])
    expect('three scenes left', len(dm.call('GET', f'{vtt}/scenes')[1]['entities']), 3)

    print('roles')
    status(dm, 'PATCH', f'/campaigns/{cid}/members/{members["ttrpg_user2"]}', 200, 'makes ttrpg_user2 a DM', {'role': 'dm'})
    got(s_p2, 'a reload as their new role', typed('reload'))
    expect('their stream ends', s_p2.ended(), True)
    s_p2 = dm_stream_of_p2 = Stream(p2, cid, after=p2.call('GET', f'{vtt}/state')[1]['seq'])
    command(dm, 200, 'moves the hidden ambush', type='token.move', token_id=ambush['id'], x=13, y=5)
    got(s_p2, 'the DM layer, as a DM now', typed('token.moved', token_id=ambush['id']))
    status(dm, 'PATCH', f'/campaigns/{cid}/members/{members["ttrpg_user2"]}', 200, 'makes them a player again', {'role': 'player'})
    got(s_p2, 'a reload as a player', typed('reload'))
    s_p2 = Stream(p2, cid, after=p2.call('GET', f'{vtt}/state')[1]['seq'])

    print('leaving takes characters and tokens along')
    status(p1, 'DELETE', f'/campaigns/{cid}/members/{members["ttrpg_user"]}', 200, 'leaves')
    got(s_dm, 'Ismark\'s token removed', typed('token.removed', token_id=t1['id']))
    got(s_dm, 'that they left', typed('member.left', user_id=members['ttrpg_user']))
    got(s_p1, 'that they left', typed('member.left', user_id=members['ttrpg_user']))
    expect('their stream ends', s_p1.ended(), True)
    status(dm, 'DELETE', f'/campaigns/{cid}/members/{members["ttrpg_user2"]}', 200, 'removes ttrpg_user2')
    got(s_dm, 'the raven losing its controller', typed('token.updated', id=raven['id'], controller_character_id=None))
    got(s_p2, 'that they were removed', typed('member.left', user_id=members['ttrpg_user2']))
    expect('their stream ends', s_p2.ended(), True)

    print('what the players never received, on any stream opened as a player')
    # ttrpg_user2's stream while they were a DM may see everything; every other player stream may not
    as_player = [s for s in Stream.opened if s.user.name != 'ttrpg_dm' and s is not dm_stream_of_p2]
    expect('player streams that were sent tokens at all (so the next check means something)',
           sum('token.added' in s.everything() for s in as_player) >= 2, True)
    secrets = ('Vampire Spawn', 'Ambush', 'Skeleton', 'previous_scene_id', 'player_name', 'player_disposition')
    leaked = sorted({f'{s.user.name}: {x}' for s in as_player for x in secrets if x in s.everything()})
    expect('true names or DM-only fields in a player\'s stream', leaked, [])

    print('deleting the campaign closes the table')
    status(dm, 'DELETE', f'/campaigns/{cid}', 200, 'deletes the campaign')
    got(s_dm, 'campaign.deleted', typed('campaign.deleted'))
    expect('the DM\'s stream ends', s_dm.ended(), True)

    print('ALL AS EXPECTED' if not failures else f'{failures} unexpected')
    return 1 if failures else 0


if __name__ == '__main__':
    sys.exit(main())
