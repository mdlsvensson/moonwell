#if DEBUG
using System;
using System.Collections.Generic;
using Moonwell.Hosting;

namespace Moonwell
{
    /// <summary>
    /// What the debug mode draws on the map for the selected object; a view adds its drawings to it. Drawing goes in frames
    /// (Begin, the calls, End): a frame reuses the last frame's lines and markers in order, moving them, draws what is new,
    /// and removes the rest, so a drawing that follows its object costs one move a line.
    /// </summary>
    internal sealed class DebugDraw
    {
        /// <summary>A plain line: ranges, cells, outlines.</summary>
        internal const int Neutral = 0;

        /// <summary>A pair that ticks.</summary>
        internal const int Ticking = 1;

        /// <summary>A pair that waits or has a cooldown running.</summary>
        internal const int Waiting = 2;

        /// <summary>A paused pair; drawn as Waiting is, as the game has only three lasting lightning codes, and the panel says paused.</summary>
        internal const int Paused = 3;

        /// <summary>The segments of a circle.</summary>
        private const int CircleSegments = 24;

        /// <summary>The debug world that draws.</summary>
        private readonly IDebugWorld world;

        /// <summary>The numbers of the lines drawn, in the order the frames drew them.</summary>
        private readonly List<int> lines = new List<int>();

        /// <summary>The style of each line in lines.</summary>
        private readonly List<int> styles = new List<int>();

        /// <summary>The numbers of the markers put, in the order the frames put them.</summary>
        private readonly List<int> markers = new List<int>();

        /// <summary>How many lines the frame running has drawn.</summary>
        private int linesUsed;

        /// <summary>How many markers the frame running has put.</summary>
        private int markersUsed;

        /// <summary>Makes the drawing of a debug world, with nothing drawn.</summary>
        internal DebugDraw(IDebugWorld world)
        {
            this.world = world;
        }

        /// <summary>Starts a frame of drawing: the calls up to End draw it.</summary>
        internal void Begin()
        {
            linesUsed = 0;
            markersUsed = 0;
        }

        /// <summary>A line in a style between two ground points: the last frame's line at this place moved, when its style is the same; else drawn.</summary>
        internal void Line(int style, float x1, float y1, float x2, float y2)
        {
            if (linesUsed < lines.Count)
            {
                if (styles[linesUsed] == style)
                {
                    world.MoveLine(lines[linesUsed], x1, y1, x2, y2);
                }
                else
                {
                    world.RemoveLine(lines[linesUsed]);
                    lines[linesUsed] = world.AddLine(style, x1, y1, x2, y2);
                    styles[linesUsed] = style;
                }
            }
            else
            {
                lines.Add(world.AddLine(style, x1, y1, x2, y2));
                styles.Add(style);
            }

            linesUsed++;
        }

        /// <summary>A circle in a style around a ground point, in 24 lines.</summary>
        internal void Circle(int style, float x, float y, float radius)
        {
            var step = 2.0f * MathF.PI / CircleSegments;
            var lastX = x + radius;
            var lastY = y;
            for (var i = 1; i <= CircleSegments; i++)
            {
                var angle = step * i;
                var nextX = x + radius * MathF.Cos(angle);
                var nextY = y + radius * MathF.Sin(angle);
                Line(style, lastX, lastY, nextX, nextY);
                lastX = nextX;
                lastY = nextY;
            }
        }

        /// <summary>A marker on a ground point: the last frame's marker at this place moved; else put.</summary>
        internal void Marker(float x, float y)
        {
            if (markersUsed < markers.Count)
            {
                world.MoveMarker(markers[markersUsed], x, y);
            }
            else
            {
                markers.Add(world.AddMarker(x, y));
            }

            markersUsed++;
        }

        /// <summary>Ends a frame: the last frame's lines and markers this one did not reuse are removed.</summary>
        internal void End()
        {
            while (lines.Count > linesUsed)
            {
                var last = lines.Count - 1;
                world.RemoveLine(lines[last]);
                lines.RemoveAt(last);
                styles.RemoveAt(last);
            }

            while (markers.Count > markersUsed)
            {
                var last = markers.Count - 1;
                world.RemoveMarker(markers[last]);
                markers.RemoveAt(last);
            }
        }

        /// <summary>Removes every line and marker: an empty frame.</summary>
        internal void Clear()
        {
            Begin();
            End();
        }
    }
}
#endif
