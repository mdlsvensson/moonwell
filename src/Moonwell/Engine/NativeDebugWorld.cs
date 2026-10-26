#if DEBUG
using System;
using System.Collections.Generic;
using System.Diagnostics;
using Moonwell.Hosting;

namespace Moonwell.Engine
{
    /// <summary>The game as the debug mode sees it, through the game's natives, in a development build: its lines and markers are kept in lists of their own, by number.</summary>
    internal sealed class NativeDebugWorld : IDebugWorld
    {
        /// <summary>The marker's model: the rally point's flag, which shows on any ground.</summary>
        private const string MarkerModel = "UI\\Feedback\\RallyPoint\\RallyPoint.mdl";

        /// <summary>How far over the ground lines are drawn, so they are not hidden in it.</summary>
        private const float LineHeight = 8.0f;

        /// <summary>The x of the panel's top right corner, in the screen's frame units.</summary>
        private const float PanelRight = 0.8f;

        /// <summary>The y of the panel's top right corner: below the top bar's gold, food and upkeep.</summary>
        private const float PanelTop = 0.575f;

        /// <summary>The panel's width.</summary>
        private const float PanelWidth = 0.22f;

        /// <summary>The panel's height, which holds 30 lines.</summary>
        private const float PanelHeight = 0.36f;

        /// <summary>How far the text is in from each of the panel's edges.</summary>
        private const float PanelInset = 0.012f;

        /// <summary>The native world, whose clock gives the game's time.</summary>
        private readonly NativeWorld native;

        /// <summary>The lines drawn, by their numbers; null where a line was removed.</summary>
        private readonly List<lightning?> lines = new List<lightning?>();

        /// <summary>The numbers removed lines left, to give again.</summary>
        private readonly List<int> freeLines = new List<int>();

        /// <summary>The markers put, by their numbers; null where a marker was removed.</summary>
        private readonly List<effect?> markers = new List<effect?>();

        /// <summary>The numbers removed markers left, to give again.</summary>
        private readonly List<int> freeMarkers = new List<int>();

        /// <summary>The panel's backdrop; null until the first ShowPanel.</summary>
        private framehandle? panel;

        /// <summary>The panel's text frame; null until the first ShowPanel.</summary>
        private framehandle? panelText;

        /// <summary>The location the ground's height is read through; made on the first read.</summary>
        private location? spot;

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
        /// pressed; H let go with every meta, so the session can tell a held H's repeats; left Ctrl with every meta, pressed
        /// and let go (held, it repeats with meta 2, and its release comes with meta 0). One more for every slot's mouse
        /// presses, which hands on the left button's.
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
                    Natives.BlzTriggerRegisterPlayerKeyEvent(keys, p, Natives.OSKEY_H, meta, false);
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

        /// <summary>Raw game seconds since the runtime started, from the native world's clock, before Moonwell's clock halts or scales them.</summary>
        public float GameNow()
        {
            return native.GameNow();
        }

        /// <summary>Draws a line of lightning of the style's code between two ground points, a little over the ground, and returns its place in the lines; a place a removed line left is given again.</summary>
        public int AddLine(int style, float x1, float y1, float x2, float y2)
        {
            var bolt = Natives.AddLightningEx(CodeOf(style), false, x1, y1, GroundZ(x1, y1), x2, y2, GroundZ(x2, y2));
            if (freeLines.Count > 0)
            {
                var place = freeLines[freeLines.Count - 1];
                freeLines.RemoveAt(freeLines.Count - 1);
                lines[place] = bolt;
                return place;
            }

            lines.Add(bolt);
            return lines.Count - 1;
        }

        /// <summary>Moves a line to two other ground points: MoveLightningEx.</summary>
        public void MoveLine(int line, float x1, float y1, float x2, float y2)
        {
            var bolt = lines[line];
            if (bolt != null)
            {
                Natives.MoveLightningEx(bolt, false, x1, y1, GroundZ(x1, y1), x2, y2, GroundZ(x2, y2));
            }
        }

        /// <summary>Removes a line: DestroyLightning; its place is given to a later line.</summary>
        public void RemoveLine(int line)
        {
            var bolt = lines[line];
            if (bolt != null)
            {
                Natives.DestroyLightning(bolt);
                lines[line] = null;
                freeLines.Add(line);
            }
        }

        /// <summary>Puts the marker model on a ground point and returns its place in the markers; a place a removed marker left is given again.</summary>
        public int AddMarker(float x, float y)
        {
            var effect = Natives.AddSpecialEffect(MarkerModel, x, y);
            if (freeMarkers.Count > 0)
            {
                var place = freeMarkers[freeMarkers.Count - 1];
                freeMarkers.RemoveAt(freeMarkers.Count - 1);
                markers[place] = effect;
                return place;
            }

            markers.Add(effect);
            return markers.Count - 1;
        }

        /// <summary>Moves a marker to another ground point, on the ground there.</summary>
        public void MoveMarker(int marker, float x, float y)
        {
            var effect = markers[marker];
            if (effect != null)
            {
                Natives.BlzSetSpecialEffectPosition(effect, x, y, GroundZ(x, y) - LineHeight);
            }
        }

        /// <summary>Removes a marker at once, with no death animation: BlzRemoveEffect; its place is given to a later marker.</summary>
        public void RemoveMarker(int marker)
        {
            var effect = markers[marker];
            if (effect != null)
            {
                Natives.BlzRemoveEffect(effect);
                markers[marker] = null;
                freeMarkers.Add(marker);
            }
        }

        /// <summary>
        /// Shows or hides the panel. The first call makes its frames, on the game's UI: a backdrop (Textures\Black32.blp at
        /// alpha 180) with its top right corner at (0.8, 0.575), 0.22 by 0.36, below the top bar and clear of the minimap and
        /// command card (game facts), and a text frame 0.012 in from each of its edges, which holds 30 lines. They are never
        /// destroyed: destroying a frame can desync (a claim in game facts).
        /// </summary>
        public void ShowPanel(bool shown)
        {
            if (panel == null)
            {
                var root = Natives.BlzGetOriginFrame(Natives.ORIGIN_FRAME_GAME_UI, 0);
                var back = Natives.BlzCreateFrameByType("BACKDROP", "MoonwellDebugPanel", root, "", 0);
                Natives.BlzFrameSetTexture(back, "Textures\\Black32.blp", 0, true);
                Natives.BlzFrameSetAlpha(back, 180);
                Natives.BlzFrameSetAbsPoint(back, Natives.FRAMEPOINT_TOPRIGHT, PanelRight, PanelTop);
                Natives.BlzFrameSetSize(back, PanelWidth, PanelHeight);
                var text = Natives.BlzCreateFrameByType("TEXT", "MoonwellDebugPanelText", back, "", 0);
                Natives.BlzFrameSetPoint(text, Natives.FRAMEPOINT_TOPLEFT, back, Natives.FRAMEPOINT_TOPLEFT, PanelInset, -PanelInset);
                Natives.BlzFrameSetPoint(text, Natives.FRAMEPOINT_BOTTOMRIGHT, back, Natives.FRAMEPOINT_BOTTOMRIGHT, -PanelInset, PanelInset);
                Natives.BlzFrameSetTextAlignment(text, Natives.TEXT_JUSTIFY_TOP, Natives.TEXT_JUSTIFY_LEFT);
                panel = back;
                panelText = text;
            }

            Natives.BlzFrameSetVisible(panel, shown);
        }

        /// <summary>Sets the text frame's text; nothing before the first ShowPanel.</summary>
        public void SetPanelText(string text)
        {
            if (panelText != null)
            {
                Natives.BlzFrameSetText(panelText, text);
            }
        }

        /// <summary>Appends a line to the log through the native world's log (CustomMapData\Moonwell\log.txt).</summary>
        public void Log(string text)
        {
            native.Log(text);
        }

        /// <summary>The lightning code of a DebugDraw style: three codes last when moved (game facts), so Paused draws as Waiting does.</summary>
        private static string CodeOf(int style)
        {
            if (style == DebugDraw.Ticking)
            {
                return "DRAL";
            }

            if (style == DebugDraw.Waiting || style == DebugDraw.Paused)
            {
                return "DRAM";
            }

            return "LEAS";
        }

        /// <summary>The height a line's end is drawn at over a ground point: the ground's, read through one reused location, and a little over it. The height is only drawn with, never decides anything, so a machine whose ground reads otherwise draws otherwise and nothing more.</summary>
        private float GroundZ(float x, float y)
        {
            if (spot == null)
            {
                spot = Natives.Location(x, y);
            }
            else
            {
                Natives.MoveLocation(spot, x, y);
            }

            return Natives.GetLocationZ(spot) + LineHeight;
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
