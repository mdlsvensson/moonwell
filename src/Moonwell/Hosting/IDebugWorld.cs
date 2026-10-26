#if DEBUG
using System;

namespace Moonwell.Hosting
{
    /// <summary>The game as the debug mode sees it, in a development build: its chat command, keys and mouse, holding units, timing, drawing, the panel and the log; the game implements it, and the tests fake it.</summary>
    public interface IDebugWorld
    {
        /// <summary>Starts delivering every player slot's chat lines that start with "-" to the action, with the slot; called once, at the start.</summary>
        void ListenChat(Action<int, string> heard);

        /// <summary>
        /// Starts delivering every player slot's debug keys and left presses: H, N, "-" and "=" pressed with Ctrl (meta 2) or
        /// Ctrl + Shift (meta 3), H let go with any meta, and left Ctrl pressed and let go with any meta, to the key action
        /// with the slot, the key (DebugKey), the meta and whether it is down; the left mouse button's press to the press
        /// action with the slot and its ground point. Called once, at the first -debug.
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

        /// <summary>Raw game seconds since the runtime started, before Moonwell's clock halts or scales them.</summary>
        float GameNow();

        /// <summary>Draws a line of lightning in a style (DebugDraw's Neutral, Ticking, Waiting or Paused) between two ground points, and returns its number.</summary>
        int AddLine(int style, float x1, float y1, float x2, float y2);

        /// <summary>Moves a line drawn by AddLine to two other ground points.</summary>
        void MoveLine(int line, float x1, float y1, float x2, float y2);

        /// <summary>Removes a line drawn by AddLine; its number may be given to a later line.</summary>
        void RemoveLine(int line);

        /// <summary>Puts a marker on a ground point, and returns its number.</summary>
        int AddMarker(float x, float y);

        /// <summary>Moves a marker put by AddMarker to another ground point.</summary>
        void MoveMarker(int marker, float x, float y);

        /// <summary>Removes a marker put by AddMarker at once; its number may be given to a later marker.</summary>
        void RemoveMarker(int marker);

        /// <summary>Shows or hides the panel; its frames are made on the first call, and only hidden and shown after, never destroyed.</summary>
        void ShowPanel(bool shown);

        /// <summary>Sets the panel's text, lines split by "\n"; nothing before the panel's frames are made.</summary>
        void SetPanelText(string text);

        /// <summary>Appends a line to the log, as the world's Log does.</summary>
        void Log(string text);
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
