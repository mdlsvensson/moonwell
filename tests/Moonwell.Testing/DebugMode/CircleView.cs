#if DEBUG
using System.Collections.Generic;

namespace Moonwell.Testing
{
    /// <summary>A view for the debug mode's panel cases: a circle of 100 around a selected unit, one line, and one line of statistics.</summary>
    internal sealed class CircleView : IDebugView
    {
        /// <summary>Adds nothing: units are the session's.</summary>
        public void Candidates(float x, float y, float radius, List<Actor> into)
        {
        }

        /// <summary>Knows no place: units are the session's.</summary>
        public bool Place(Actor actor, out float x, out float y)
        {
            x = 0.0f;
            y = 0.0f;
            return false;
        }

        /// <summary>Adds "circle 100".</summary>
        public void Lines(Actor selected, DebugLines lines)
        {
            lines.Add("circle 100");
        }

        /// <summary>A circle of 100 around a selected unit, Neutral.</summary>
        public void Draw(Actor selected, DebugDraw draw)
        {
            if (selected is Unit unit)
            {
                draw.Circle(DebugDraw.Neutral, unit.X, unit.Y, 100.0f);
            }
        }

        /// <summary>Adds "circles 1".</summary>
        public void Stats(DebugLines lines)
        {
            lines.Add("circles 1");
        }
    }
}
#endif
