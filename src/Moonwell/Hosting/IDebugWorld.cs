#if DEBUG
using System;

namespace Moonwell.Hosting
{
    /// <summary>The game as the debug mode sees it, in a development build: its chat command, keys and mouse, holding units, and timing; the game implements it, and the tests fake it.</summary>
    public interface IDebugWorld
    {
        /// <summary>Starts delivering every player slot's chat lines that start with "-" to the action, with the slot; called once, at the start.</summary>
        void ListenChat(Action<int, string> heard);

        /// <summary>
        /// Starts delivering every player slot's debug keys and left presses: H, N, "-" and "=" pressed with Ctrl (meta 2) or
        /// Ctrl + Shift (meta 3), and left Ctrl pressed and let go with any meta, to the key action with the slot, the key
        /// (DebugKey), the meta and whether it is down; the left mouse button's press to the press action with the slot and
        /// its ground point. Called once, at the first -debug.
        /// </summary>
        void ListenInput(Action<int, int, int, bool> key, Action<int, float, float> leftPress);

        /// <summary>Whether a unit is paused (PauseUnit's flag).</summary>
        bool IsPaused(unit unit);

        /// <summary>Pauses a unit or lets it go (PauseUnit).</summary>
        void Pause(unit unit, bool paused);

        /// <summary>Calls the action once after the seconds of game time, on a game timer of its own; Moonwell's clock does not move it.</summary>
        void After(float seconds, Action action);

        /// <summary>Real seconds of this machine's processor (os.clock), for timing; never the same on two machines.</summary>
        float Clock();

        /// <summary>The game's seconds since the runtime started, under Moonwell's clock.</summary>
        float GameNow();
    }

    /// <summary>The debug keys as the debug world hands them to the session.</summary>
    internal static class DebugKey
    {
        /// <summary>H: halt or resume.</summary>
        internal const int H = 1;

        /// <summary>N: one step.</summary>
        internal const int N = 2;

        /// <summary>"-": slower.</summary>
        internal const int Minus = 3;

        /// <summary>"=": faster.</summary>
        internal const int Plus = 4;

        /// <summary>Left Ctrl.</summary>
        internal const int Control = 5;
    }
}
#endif
