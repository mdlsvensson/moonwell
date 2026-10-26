#if DEBUG
using System;
using System.Collections.Generic;
using Moonwell.Hosting;

namespace Moonwell.Testing
{
    /// <summary>The fake debug world a debug-mode case runs against: it records what the session asks of it, and its chat, keys, presses and time drive the session.</summary>
    public sealed class FakeDebugWorld : IDebugWorld
    {
        /// <summary>The fake world under it, whose units and time it shares.</summary>
        private readonly FakeWorld world;

        /// <summary>How many times the session asked to hear the chat.</summary>
        public int ChatListens;

        /// <summary>How many times the session asked to hear the keys and the mouse.</summary>
        public int InputListens;

        /// <summary>The units the session paused or let go, in order, with a "pause" or "let go" each in PauseCalls.</summary>
        public List<unit> PausedOrLetGo = new List<unit>();

        /// <summary>"pause" or "let go" for each unit in PausedOrLetGo.</summary>
        public List<string> PauseCalls = new List<string>();

        /// <summary>What Clock reads: os.clock's seconds in the game.</summary>
        public float ClockSeconds;

        /// <summary>The chat listener, once the session has asked.</summary>
        private Action<int, string>? heard;

        /// <summary>The key listener, once the session has asked.</summary>
        private Action<int, int, int, bool>? keyHeard;

        /// <summary>The left press listener, once the session has asked.</summary>
        private Action<int, float, float>? pressed;

        /// <summary>The game time each waiting After callback is due, in the order they were asked for.</summary>
        private readonly List<float> afterDues = new List<float>();

        /// <summary>The waiting After callbacks, beside their due times.</summary>
        private readonly List<Action> afterActions = new List<Action>();

        /// <summary>Makes a fake debug world over a fake world.</summary>
        public FakeDebugWorld(FakeWorld world)
        {
            this.world = world;
        }

        /// <summary>Makes a fake debug world over a fake world and installs it as the session's, as Runtime.Start installs the native one.</summary>
        public static FakeDebugWorld Install(FakeWorld world)
        {
            var debug = new FakeDebugWorld(world);
            DebugSession.Install(debug);
            return debug;
        }

        /// <summary>Records the chat listener.</summary>
        public void ListenChat(Action<int, string> heard)
        {
            ChatListens++;
            this.heard = heard;
        }

        /// <summary>Records the key and left press listeners.</summary>
        public void ListenInput(Action<int, int, int, bool> key, Action<int, float, float> leftPress)
        {
            InputListens++;
            keyHeard = key;
            pressed = leftPress;
        }

        /// <summary>Whether a unit is paused, from the fake world.</summary>
        public bool IsPaused(unit unit)
        {
            return world.IsPausedUnit(unit);
        }

        /// <summary>Pauses or lets go a unit in the fake world, and records it.</summary>
        public void Pause(unit unit, bool paused)
        {
            PausedOrLetGo.Add(unit);
            PauseCalls.Add(paused ? "pause" : "let go");
            world.SetPaused(unit, paused);
        }

        /// <summary>Keeps a callback for the seconds of game time from now; Advance runs it.</summary>
        public void After(float seconds, Action action)
        {
            afterDues.Add(world.Time + seconds);
            afterActions.Add(action);
        }

        /// <summary>The seconds ClockSeconds holds.</summary>
        public float Clock()
        {
            return ClockSeconds;
        }

        /// <summary>The fake world's game time.</summary>
        public float GameNow()
        {
            return world.Time;
        }

        /// <summary>A player's chat line: heard once the session listens, when the text starts with "-", untrimmed, as the native world passes on.</summary>
        public void Chat(int slot, string text)
        {
            if (heard != null && text.StartsWith("-"))
            {
                heard(slot, text);
            }
        }

        /// <summary>A player's key event: heard when the session listens for that key and meta, as the native world registers them.</summary>
        public void Key(int slot, int key, int meta, bool down)
        {
            if (keyHeard == null)
            {
                return;
            }

            var registered = key == DebugKey.Control || (key == DebugKey.H && !down) || (down && (meta == 2 || meta == 3));
            if (registered)
            {
                keyHeard(slot, key, meta, down);
            }
        }

        /// <summary>A player presses Ctrl and a key, with Shift too when asked, and lets go of the key and of Ctrl.</summary>
        public void Ctrl(int slot, int key, bool shift = false)
        {
            Key(slot, DebugKey.Control, 0, true);
            Key(slot, key, shift ? 3 : 2, true);
            Key(slot, key, shift ? 3 : 2, false);
            Key(slot, DebugKey.Control, 0, false);
        }

        /// <summary>A player's left press at a ground point, heard once the session listens.</summary>
        public void Press(int slot, float x, float y)
        {
            pressed?.Invoke(slot, x, y);
        }

        /// <summary>A player's Ctrl + left click at a ground point.</summary>
        public void CtrlClick(int slot, float x, float y)
        {
            Key(slot, DebugKey.Control, 0, true);
            Press(slot, x, y);
            Key(slot, DebugKey.Control, 0, false);
        }

        /// <summary>Moves game time forward by the seconds: the fake world's wakes, and each After callback at its time between them, in due order, those asked for on the way too.</summary>
        public void Advance(float seconds)
        {
            var target = world.Time + seconds;
            while (true)
            {
                var next = -1;
                for (var i = 0; i < afterDues.Count; i++)
                {
                    if (afterDues[i] <= target + 0.0001f && (next < 0 || afterDues[i] < afterDues[next]))
                    {
                        next = i;
                    }
                }

                if (next < 0)
                {
                    break;
                }

                var due = afterDues[next];
                var action = afterActions[next];
                afterDues.RemoveAt(next);
                afterActions.RemoveAt(next);
                if (due > world.Time)
                {
                    world.Advance(due - world.Time);
                }

                action();
            }

            if (target > world.Time)
            {
                world.Advance(target - world.Time);
            }
        }
    }
}
#endif
