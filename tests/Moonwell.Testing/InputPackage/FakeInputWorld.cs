using System;
using System.Collections.Generic;
using Moonwell.Hosting;

namespace Moonwell.Testing
{
    /// <summary>The fake game the input package's cases run against: mouse events a case sends by hand, the local reads a case sets, and a record of the changes the package made for a player's screen.</summary>
    public sealed class FakeInputWorld : IInputWorld
    {
        /// <summary>The three actions the package gave for one player's mouse.</summary>
        public sealed class Listened
        {
            /// <summary>The player the package listened for.</summary>
            public player Player = new player();

            /// <summary>What the package runs on a move.</summary>
            public Action? Moved;

            /// <summary>What the package runs on a press.</summary>
            public Action? Pressed;

            /// <summary>What the package runs on a release.</summary>
            public Action? Released;
        }

        /// <summary>Every ListenMouse call, in order.</summary>
        public List<Listened> Listens = new List<Listened>();

        /// <summary>The world's maximum x, as WorldTopRight gives it.</summary>
        public float MaxX = 4096f;

        /// <summary>The world's maximum y, as WorldTopRight gives it.</summary>
        public float MaxY = 4096f;

        /// <summary>How many times WorldTopRight was called.</summary>
        public int WorldReads;

        /// <summary>The cursor's screen x on this machine.</summary>
        public int ScreenX;

        /// <summary>The cursor's screen y on this machine.</summary>
        public int ScreenY;

        /// <summary>The unit under the cursor on this machine; null for none.</summary>
        public unit? Focus;

        /// <summary>Which buttons (1 to 3) are held on this machine now.</summary>
        public bool[] Held = new bool[4];

        /// <summary>The changes for a screen, as "screen slot x y" and "cursor slot show" or "cursor slot hide", in order.</summary>
        public List<string> Calls = new List<string>();

        /// <summary>One change the package made to a player's camera: its kind ("field", "pan", "follow", "stop", "reset", "bounds" or "apply"), the slot, and what the kind uses of the rest.</summary>
        public sealed class CameraCall
        {
            /// <summary>What changed.</summary>
            public string Kind = "";

            /// <summary>The slot of the player whose camera changed.</summary>
            public int Slot;

            /// <summary>The field code of a "field" change.</summary>
            public int Field;

            /// <summary>A field's value; a pan's x; a bounds' minimum x.</summary>
            public float A;

            /// <summary>A pan's y; a bounds' minimum y.</summary>
            public float B;

            /// <summary>A bounds' maximum x.</summary>
            public float C;

            /// <summary>A bounds' maximum y.</summary>
            public float D;

            /// <summary>The seconds a change takes, where it has them.</summary>
            public float Seconds;

            /// <summary>The unit a "follow" change tracks.</summary>
            public unit? Unit;

            /// <summary>The setup an "apply" change used.</summary>
            public camerasetup? Setup;
        }

        /// <summary>One setup the package made: what the game's own script would have set on it.</summary>
        public sealed class MadeSetup
        {
            /// <summary>The handle the package got back.</summary>
            public camerasetup Setup = new camerasetup();

            /// <summary>The destination x.</summary>
            public float X;

            /// <summary>The destination y.</summary>
            public float Y;

            /// <summary>The field codes set, in order.</summary>
            public List<int> Codes = new List<int>();

            /// <summary>The values set, in the codes' order.</summary>
            public List<float> Values = new List<float>();

            /// <summary>The camera type set, or -1 when none was.</summary>
            public int Type = -1;
        }

        /// <summary>This machine's camera fields by Moonwell's code (0 to 13), in the game's units: radians for the angles.</summary>
        public float[] Fields = new float[14];

        /// <summary>This machine's camera target.</summary>
        public float TargetX;

        /// <summary>This machine's camera target y.</summary>
        public float TargetY;

        /// <summary>This machine's camera target z.</summary>
        public float TargetZ;

        /// <summary>This machine's camera eye x.</summary>
        public float EyeX;

        /// <summary>This machine's camera eye y.</summary>
        public float EyeY;

        /// <summary>This machine's camera eye z.</summary>
        public float EyeZ;

        /// <summary>Every change to a player's camera, in order.</summary>
        public List<CameraCall> CameraCalls = new List<CameraCall>();

        /// <summary>Every setup the package made, in order.</summary>
        public List<MadeSetup> Made = new List<MadeSetup>();

        /// <summary>The player of the event being delivered.</summary>
        private player? eventPlayer;

        /// <summary>The x of the event being delivered.</summary>
        private float eventX;

        /// <summary>The y of the event being delivered.</summary>
        private float eventY;

        /// <summary>The button of the event being delivered.</summary>
        private int eventButton;

        /// <summary>Sends a move event for a player's mouse; the player must have been listened for.</summary>
        public void Move(Player player, float x, float y)
        {
            Send(player, x, y, 0, 0);
        }

        /// <summary>Sends a press event for a player's mouse.</summary>
        public void Press(Player player, MouseButton button, float x, float y)
        {
            Send(player, x, y, (int)button, 1);
        }

        /// <summary>Sends a release event for a player's mouse.</summary>
        public void Release(Player player, MouseButton button, float x, float y)
        {
            Send(player, x, y, (int)button, 2);
        }

        /// <summary>Runs the action of a kind (0 move, 1 press, 2 release) with the event's values set, and puts the previous ones back.</summary>
        private void Send(Player player, float x, float y, int button, int kind)
        {
            Listened? found = null;
            for (var i = 0; i < Listens.Count; i++)
            {
                if (Listens[i].Player == player.Handle)
                {
                    found = Listens[i];
                }
            }

            if (found == null)
            {
                throw new ExpectationFailed("The package did not listen for slot " + player.Slot + ".");
            }

            var beforePlayer = eventPlayer;
            var beforeX = eventX;
            var beforeY = eventY;
            var beforeButton = eventButton;
            eventPlayer = player.Handle;
            eventX = x;
            eventY = y;
            eventButton = button;
            try
            {
                var action = kind == 0 ? found.Moved : kind == 1 ? found.Pressed : found.Released;
                if (action != null)
                {
                    action();
                }
            }
            finally
            {
                eventPlayer = beforePlayer;
                eventX = beforeX;
                eventY = beforeY;
                eventButton = beforeButton;
            }
        }

        /// <summary>Remembers what to run for a player's mouse.</summary>
        public void ListenMouse(player p, Action moved, Action pressed, Action released)
        {
            var one = new Listened();
            one.Player = p;
            one.Moved = moved;
            one.Pressed = pressed;
            one.Released = released;
            Listens.Add(one);
        }

        /// <summary>The player of the event being delivered.</summary>
        public player EventPlayer() => eventPlayer!;

        /// <summary>The x of the event being delivered.</summary>
        public float EventMouseX() => eventX;

        /// <summary>The y of the event being delivered.</summary>
        public float EventMouseY() => eventY;

        /// <summary>The button of the event being delivered.</summary>
        public int EventMouseButton() => eventButton;

        /// <summary>The world's top-right corner a case set.</summary>
        public void WorldTopRight(out float maxX, out float maxY)
        {
            WorldReads++;
            maxX = MaxX;
            maxY = MaxY;
        }

        /// <summary>The screen x a case set.</summary>
        public int MouseScreenX() => ScreenX;

        /// <summary>The screen y a case set.</summary>
        public int MouseScreenY() => ScreenY;

        /// <summary>The unit a case put under the cursor.</summary>
        public unit? MouseFocusUnit() => Focus;

        /// <summary>Whether a case holds the button.</summary>
        public bool IsMouseButtonPressed(int button) => Held[button];

        /// <summary>Remembers the cursor move.</summary>
        public void SetMouseScreen(player p, int x, int y)
        {
            Calls.Add("screen " + Player.Of(p)!.Slot + " " + x + " " + y);
        }

        /// <summary>Remembers the cursor change.</summary>
        public void ShowCursor(player p, bool show)
        {
            Calls.Add("cursor " + Player.Of(p)!.Slot + " " + (show ? "show" : "hide"));
        }

        /// <summary>The field a case set.</summary>
        public float CameraField(int field) => Fields[field];

        /// <summary>The target x a case set.</summary>
        public float CameraTargetX() => TargetX;

        /// <summary>The target y a case set.</summary>
        public float CameraTargetY() => TargetY;

        /// <summary>The target z a case set.</summary>
        public float CameraTargetZ() => TargetZ;

        /// <summary>The eye x a case set.</summary>
        public float CameraEyeX() => EyeX;

        /// <summary>The eye y a case set.</summary>
        public float CameraEyeY() => EyeY;

        /// <summary>The eye z a case set.</summary>
        public float CameraEyeZ() => EyeZ;

        /// <summary>Remembers a camera change and returns its record.</summary>
        private CameraCall Record(string kind, player p)
        {
            var call = new CameraCall();
            call.Kind = kind;
            call.Slot = Player.Of(p)!.Slot;
            CameraCalls.Add(call);
            return call;
        }

        /// <summary>Remembers the field change.</summary>
        public void SetCameraField(player p, int field, float value, float seconds)
        {
            var call = Record("field", p);
            call.Field = field;
            call.A = value;
            call.Seconds = seconds;
        }

        /// <summary>Remembers the pan.</summary>
        public void PanCamera(player p, float x, float y, float seconds)
        {
            var call = Record("pan", p);
            call.A = x;
            call.B = y;
            call.Seconds = seconds;
        }

        /// <summary>Remembers the follow.</summary>
        public void FollowUnit(player p, unit u)
        {
            Record("follow", p).Unit = u;
        }

        /// <summary>Remembers the end of the follow.</summary>
        public void StopFollowing(player p)
        {
            Record("stop", p);
        }

        /// <summary>Remembers the reset.</summary>
        public void ResetCamera(player p, float seconds)
        {
            Record("reset", p).Seconds = seconds;
        }

        /// <summary>Remembers the bounds.</summary>
        public void SetCameraBounds(player p, float minX, float minY, float maxX, float maxY)
        {
            var call = Record("bounds", p);
            call.A = minX;
            call.B = minY;
            call.C = maxX;
            call.D = maxY;
        }

        /// <summary>Reads one camera from the data as the game's own script does: its target, the pairs, and the type, and remembers what it set.</summary>
        public camerasetup MakeSetup(float[] values, int offset)
        {
            var made = new MadeSetup();
            made.X = values[offset];
            made.Y = values[offset + 1];
            var count = (int)values[offset + 2];
            var at = offset + 3;
            for (var i = 0; i < count; i++)
            {
                made.Codes.Add((int)values[at]);
                made.Values.Add(values[at + 1]);
                at += 2;
            }

            made.Type = (int)values[at];
            Made.Add(made);
            return made.Setup;
        }

        /// <summary>Remembers the apply.</summary>
        public void ApplySetup(player p, camerasetup setup, float seconds)
        {
            var call = Record("apply", p);
            call.Setup = setup;
            call.Seconds = seconds;
        }
    }
}
