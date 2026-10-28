#if DEBUG
using System.Collections.Generic;

namespace Moonwell.Engine
{
    /// <summary>
    /// The physics package's view in the debug mode: a selected missile's speed, velocity and radius, a selected unit's
    /// push, and the counts of missiles and pushed units. It adds no objects to click (a missile is a gizmo, which the
    /// interactions package's view finds) and draws nothing; it reads the lists as they are, so selecting changes nothing.
    /// </summary>
    internal sealed class PhysicsView : IDebugView
    {
        /// <summary>The view; made on first use.</summary>
        private static PhysicsView? instance;

        /// <summary>The view, made on first use; Physics.Install adds it to the debug mode.</summary>
        internal static PhysicsView Instance
        {
            get
            {
                if (instance == null)
                {
                    instance = new PhysicsView();
                }

                return instance;
            }
        }

        /// <summary>Adds nothing: a missile is a gizmo, which the interactions package's view adds, and units are the session's.</summary>
        public void Candidates(float x, float y, float radius, List<Actor> into)
        {
        }

        /// <summary>Knows no place: the interactions package's view gives a missile's, and the session a unit's.</summary>
        public bool Place(Actor actor, out float x, out float y)
        {
            x = 0.0f;
            y = 0.0f;
            return false;
        }

        /// <summary>
        /// "speed 900 · velocity 900, 0, 0 · radius 16" for a missile; "pushed: velocity 400, 0, 0" for a pushed unit, with
        /// " · lifted 120 up" while it is off its height. Nothing for another object, or a unit not pushed.
        /// </summary>
        public void Lines(Actor selected, DebugLines lines)
        {
            var missile = selected as Missile;
            if (missile != null)
            {
                lines.Add("speed " + DebugPanel.Whole(missile.Speed) + " · velocity " + DebugPanel.Whole(missile.vx) + ", " + DebugPanel.Whole(missile.vy) + ", " + DebugPanel.Whole(missile.vz) + " · radius " + DebugPanel.Whole(missile.radius));
                return;
            }

            var unit = selected as Unit;
            if (unit == null)
            {
                return;
            }

            var push = Pushes.Find(unit);
            if (push == null)
            {
                return;
            }

            var line = "pushed: velocity " + DebugPanel.Whole(push.vx) + ", " + DebugPanel.Whole(push.vy) + ", " + DebugPanel.Whole(push.lifted ? push.vz : 0.0f);
            if (push.lifted)
            {
                line = line + " · lifted " + DebugPanel.Whole(push.height) + " up";
            }

            lines.Add(line);
        }

        /// <summary>Draws nothing: core draws the marker.</summary>
        public void Draw(Actor selected, DebugDraw draw)
        {
        }

        /// <summary>"physics: 300 missiles (12 resting) · 50 pushed units": the missiles that have not begun to end, and the units being pushed.</summary>
        public void Stats(DebugLines lines)
        {
            int resting;
            var missiles = Pass.Flying(out resting);
            lines.Add("physics: " + Plural(missiles, "missile") + " (" + DebugPanel.Count(resting) + " resting) · " + Plural(Pushes.Active, "pushed unit"));
        }

        /// <summary>"1 missile", "2 missiles": a count and a noun, plural unless the count is 1.</summary>
        private static string Plural(int count, string noun)
        {
            return DebugPanel.Count(count) + " " + noun + (count == 1 ? "" : "s");
        }
    }
}
#endif
