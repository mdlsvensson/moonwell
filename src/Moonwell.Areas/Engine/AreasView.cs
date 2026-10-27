#if DEBUG
using System.Collections.Generic;

namespace Moonwell.Engine
{
    /// <summary>
    /// The areas package's view in the debug mode: for a selected unit, the areas it is in, in the panel and drawn as
    /// outlines. It adds no objects to click and no statistics, and it reads the tracker as it is, so selecting makes none.
    /// </summary>
    internal sealed class AreasView : IDebugView
    {
        /// <summary>The view; made on first use.</summary>
        private static AreasView? instance;

        /// <summary>The view, made on first use; the tracker adds it to the debug mode when it is made.</summary>
        internal static AreasView Instance
        {
            get
            {
                if (instance == null)
                {
                    instance = new AreasView();
                }

                return instance;
            }
        }

        /// <summary>Adds nothing: the areas are not objects to select, and units are the session's.</summary>
        public void Candidates(float x, float y, float radius, List<Actor> into)
        {
        }

        /// <summary>Knows no place: the session finds a unit's place itself, and an area is not an object to select.</summary>
        public bool Place(Actor actor, out float x, out float y)
        {
            x = 0.0f;
            y = 0.0f;
            return false;
        }

        /// <summary>"in Arena since 3.5 s" for each area that has the selected unit, in the order the areas were added. Nothing for another kind of object.</summary>
        public void Lines(Actor selected, DebugLines lines)
        {
            var unit = selected as Unit;
            var tracker = AreaTracker.Existing();
            if (unit == null || tracker == null)
            {
                return;
            }

            for (var i = 0; i < tracker.AreaCount; i++)
            {
                var area = tracker.AreaAt(i);
                if (area.Has(unit))
                {
                    lines.Add("in " + Names.NameOf(area) + " since " + DebugPanel.Tenths(area.Since(unit)) + " s");
                }
            }
        }

        /// <summary>The outline of each area that has the selected unit: a rect by its four edges, a polygon by its corners, a circle in 24 segments, a shape of several parts by each part. Core draws the marker.</summary>
        public void Draw(Actor selected, DebugDraw draw)
        {
            var unit = selected as Unit;
            var tracker = AreaTracker.Existing();
            if (unit == null || tracker == null)
            {
                return;
            }

            for (var i = 0; i < tracker.AreaCount; i++)
            {
                var area = tracker.AreaAt(i);
                if (area.Has(unit))
                {
                    Outline(draw, area.ShapeOfArea());
                }
            }
        }

        /// <summary>Adds nothing: the package keeps no statistics.</summary>
        public void Stats(DebugLines lines)
        {
        }

        /// <summary>Draws the outline of a shape in the plain style; a union draws each part.</summary>
        private static void Outline(DebugDraw draw, AreaShape shape)
        {
            if (shape.Kind == AreaShape.ShapeKind.Union)
            {
                for (var i = 0; i < shape.Parts.Length; i++)
                {
                    Outline(draw, shape.Parts[i]);
                }

                return;
            }

            if (shape.Kind == AreaShape.ShapeKind.Circle)
            {
                draw.Circle(DebugDraw.Neutral, shape.Centre.X, shape.Centre.Y, shape.Radius);
                return;
            }

            if (shape.Kind == AreaShape.ShapeKind.Polygon)
            {
                var points = shape.Corners!.Points;
                for (var i = 0; i < points.Count; i++)
                {
                    var next = points[(i + 1) % points.Count];
                    draw.Line(DebugDraw.Neutral, points[i].X, points[i].Y, next.X, next.Y);
                }

                return;
            }

            var bounds = shape.Bounds;
            draw.Line(DebugDraw.Neutral, bounds.Min.X, bounds.Min.Y, bounds.Max.X, bounds.Min.Y);
            draw.Line(DebugDraw.Neutral, bounds.Max.X, bounds.Min.Y, bounds.Max.X, bounds.Max.Y);
            draw.Line(DebugDraw.Neutral, bounds.Max.X, bounds.Max.Y, bounds.Min.X, bounds.Max.Y);
            draw.Line(DebugDraw.Neutral, bounds.Min.X, bounds.Max.Y, bounds.Min.X, bounds.Min.Y);
        }
    }
}
#endif
