using Moonwell.Hosting;

namespace Moonwell.Build.Tests
{
    /// <summary>
    /// An input world that only makes camera setups, reading each one's numbers as the game's own script would, and records what it set.
    /// The rest of the world is never called, so it throws.
    /// </summary>
    internal sealed class SetupRecordingWorld : IInputWorld
    {
        /// <summary>One setup that was made: what the numbers said to set on it.</summary>
        internal sealed class MadeSetup
        {
            /// <summary>The handle given back; the game's handle types have no public constructor, so it is made by reflection.</summary>
            public camerasetup Handle = (camerasetup)Activator.CreateInstance(typeof(camerasetup), true)!;

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

        /// <summary>Every setup made, in order.</summary>
        public List<MadeSetup> Made = new List<MadeSetup>();

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
            return made.Handle;
        }

        public void ListenMouse(player p, Action? moved, Action pressed, Action released) => throw Unused();

        public player EventPlayer() => throw Unused();

        public float EventMouseX() => throw Unused();

        public float EventMouseY() => throw Unused();

        public int EventMouseButton() => throw Unused();

        public void WorldTopRight(out float maxX, out float maxY) => throw Unused();

        public int MouseScreenX() => throw Unused();

        public int MouseScreenY() => throw Unused();

        public unit? MouseFocusUnit() => throw Unused();

        public bool IsMouseButtonPressed(int button) => throw Unused();

        public void SetMouseScreen(player p, int x, int y) => throw Unused();

        public void ShowCursor(player p, bool show) => throw Unused();

        public float CameraField(int field) => throw Unused();

        public float CameraTargetX() => throw Unused();

        public float CameraTargetY() => throw Unused();

        public float CameraTargetZ() => throw Unused();

        public float CameraEyeX() => throw Unused();

        public float CameraEyeY() => throw Unused();

        public float CameraEyeZ() => throw Unused();

        public void SetField(player p, int field, float value, float seconds) => throw Unused();

        public void PanCamera(player p, float x, float y, float seconds) => throw Unused();

        public void FollowUnit(player p, unit u) => throw Unused();

        public void StopFollowing(player p) => throw Unused();

        public void ResetCamera(player p, float seconds) => throw Unused();

        public void SetBounds(player p, float minX, float minY, float maxX, float maxY) => throw Unused();

        public void ApplySetup(player p, camerasetup setup, float seconds) => throw Unused();

        private static NotSupportedException Unused() => new NotSupportedException("This world only makes camera setups.");
    }
}
