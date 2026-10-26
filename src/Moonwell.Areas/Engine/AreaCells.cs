using System;
using System.Collections.Generic;

namespace Moonwell.Engine
{
    /// <summary>
    /// The 32 by 32 cells the game is given for a polygon or a circle. A region is made of whole cells on multiples of
    /// 32 from 0 (seen in the game: the cell at (10, 10) covers 0 to 32), so a cell's centre is 16 past a multiple of 32.
    /// </summary>
    internal static class AreaCells
    {
        /// <summary>The side of a cell.</summary>
        private const float Cell = 32f;

        /// <summary>Adds the centres of every cell whose centre is inside a polygon or a circle to a list, as x and y in turn, row by row from the bottom left. A union adds those of its parts, in order; a rect or a region adds none, as it is given to the game as a rect.</summary>
        internal static void Of(AreaShape shape, List<float> centres)
        {
            if (shape.Kind == AreaShape.ShapeKind.Union)
            {
                for (var i = 0; i < shape.Parts.Length; i++)
                {
                    Of(shape.Parts[i], centres);
                }

                return;
            }

            if (shape.Kind == AreaShape.ShapeKind.Rect)
            {
                return;
            }

            var bounds = shape.Bounds;
            var firstColumn = (int)MathF.Floor(bounds.Min.X / Cell);
            var lastColumn = (int)MathF.Floor(bounds.Max.X / Cell);
            var firstRow = (int)MathF.Floor(bounds.Min.Y / Cell);
            var lastRow = (int)MathF.Floor(bounds.Max.Y / Cell);
            for (var row = firstRow; row <= lastRow; row++)
            {
                var y = row * Cell + Cell / 2f;
                for (var column = firstColumn; column <= lastColumn; column++)
                {
                    var x = column * Cell + Cell / 2f;
                    if (shape.Contains(x, y))
                    {
                        centres.Add(x);
                        centres.Add(y);
                    }
                }
            }
        }
    }
}
