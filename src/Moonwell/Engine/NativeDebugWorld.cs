#if DEBUG
using System;
using System.Diagnostics;
using Moonwell.Hosting;

namespace Moonwell.Engine
{
    /// <summary>The game as the debug mode sees it, through the game's natives, in a development build.</summary>
    internal sealed class NativeDebugWorld : IDebugWorld
    {
        /// <summary>The native world, whose clock gives the game's time.</summary>
        private readonly NativeWorld native;

        /// <summary>Makes the debug world of a native world.</summary>
        internal NativeDebugWorld(NativeWorld native)
        {
            this.native = native;
        }

        /// <summary>One trigger for every player slot's chat lines with "-" in them (a chat event that is not an exact match fires for a line that holds the text anywhere).</summary>
        public void ListenChat(Action<int, string> heard)
        {
            var trigger = Natives.CreateTrigger();
            for (var slot = 0; slot < Natives.bj_MAX_PLAYERS; slot++)
            {
                Natives.TriggerRegisterPlayerChatEvent(trigger, Natives.Player(slot), "-", false);
            }

            Natives.TriggerAddAction(trigger, () =>
            {
                try
                {
                    var text = Natives.GetEventPlayerChatString();
                    if (text.StartsWith("-"))
                    {
                        heard(Natives.GetPlayerId(Natives.GetTriggerPlayer()), text);
                    }
                }
                catch (Exception error)
                {
                    Errors.Report("Moonwell.Debug", error);
                }
            });
        }

        /// <summary>
        /// One trigger for every player slot's debug keys: H, N, "-" and "=" with Ctrl (meta 2) and Ctrl + Shift (meta 3),
        /// pressed; left Ctrl with every meta, pressed and let go (held, it repeats with meta 2, and its release comes with
        /// meta 0). One more for every slot's mouse presses, which hands on the left button's.
        /// </summary>
        public void ListenInput(Action<int, int, int, bool> key, Action<int, float, float> leftPress)
        {
            var keys = Natives.CreateTrigger();
            var mouse = Natives.CreateTrigger();
            for (var slot = 0; slot < Natives.bj_MAX_PLAYERS; slot++)
            {
                var p = Natives.Player(slot);
                for (var meta = 2; meta <= 3; meta++)
                {
                    Natives.BlzTriggerRegisterPlayerKeyEvent(keys, p, Natives.OSKEY_H, meta, true);
                    Natives.BlzTriggerRegisterPlayerKeyEvent(keys, p, Natives.OSKEY_N, meta, true);
                    Natives.BlzTriggerRegisterPlayerKeyEvent(keys, p, Natives.OSKEY_OEM_MINUS, meta, true);
                    Natives.BlzTriggerRegisterPlayerKeyEvent(keys, p, Natives.OSKEY_OEM_PLUS, meta, true);
                }

                for (var meta = 0; meta < 16; meta++)
                {
                    Natives.BlzTriggerRegisterPlayerKeyEvent(keys, p, Natives.OSKEY_LCONTROL, meta, true);
                    Natives.BlzTriggerRegisterPlayerKeyEvent(keys, p, Natives.OSKEY_LCONTROL, meta, false);
                }

                Natives.TriggerRegisterPlayerEvent(mouse, p, Natives.EVENT_PLAYER_MOUSE_DOWN);
            }

            Natives.TriggerAddAction(keys, () =>
            {
                try
                {
                    var code = KeyOf(Natives.BlzGetTriggerPlayerKey());
                    if (code != 0)
                    {
                        key(Natives.GetPlayerId(Natives.GetTriggerPlayer()), code, Natives.BlzGetTriggerPlayerMetaKey(), Natives.BlzGetTriggerPlayerIsKeyDown());
                    }
                }
                catch (Exception error)
                {
                    Errors.Report("Moonwell.Debug", error);
                }
            });
            Natives.TriggerAddAction(mouse, () =>
            {
                try
                {
                    if (Natives.BlzGetTriggerPlayerMouseButton() == Natives.MOUSE_BUTTON_TYPE_LEFT)
                    {
                        leftPress(Natives.GetPlayerId(Natives.GetTriggerPlayer()), Natives.BlzGetTriggerPlayerMouseX(), Natives.BlzGetTriggerPlayerMouseY());
                    }
                }
                catch (Exception error)
                {
                    Errors.Report("Moonwell.Debug", error);
                }
            });
        }

        /// <summary>Whether a unit is paused: IsUnitPaused.</summary>
        public bool IsPaused(unit unit)
        {
            return Natives.IsUnitPaused(unit);
        }

        /// <summary>Pauses a unit or lets it go: PauseUnit.</summary>
        public void Pause(unit unit, bool paused)
        {
            Natives.PauseUnit(unit, paused);
        }

        /// <summary>Calls the action once after the seconds of game time, on a timer of its own that is destroyed when it fires.</summary>
        public void After(float seconds, Action action)
        {
            var timer = Natives.CreateTimer();
            Natives.TimerStart(timer, seconds, false, () =>
            {
                Natives.DestroyTimer(timer);
                try
                {
                    action();
                }
                catch (Exception error)
                {
                    Errors.Report("Moonwell.Debug", error);
                }
            });
        }

        /// <summary>Real seconds of this machine's processor: os.clock.</summary>
        public float Clock()
        {
            return OsClock();
        }

        /// <summary>The game's seconds since the runtime started, from the native world's clock.</summary>
        public float GameNow()
        {
            return native.GameNow();
        }

        /// <summary>Real seconds of this machine's processor: os.clock in the game, a stopwatch in .NET.</summary>
        /// @CSharpLua.Template = "os.clock()"
        private static float OsClock()
        {
            return (float)Stopwatch.GetTimestamp() / Stopwatch.Frequency;
        }

        /// <summary>The DebugKey of a game key; 0 for a key the debug mode does not listen to.</summary>
        private static int KeyOf(oskeytype key)
        {
            if (key == Natives.OSKEY_H)
            {
                return DebugKey.H;
            }

            if (key == Natives.OSKEY_N)
            {
                return DebugKey.N;
            }

            if (key == Natives.OSKEY_OEM_MINUS)
            {
                return DebugKey.Minus;
            }

            if (key == Natives.OSKEY_OEM_PLUS)
            {
                return DebugKey.Plus;
            }

            return key == Natives.OSKEY_LCONTROL ? DebugKey.Control : 0;
        }
    }
}
#endif
