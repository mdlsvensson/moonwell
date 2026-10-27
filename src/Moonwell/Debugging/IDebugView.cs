#if DEBUG
using System.Collections.Generic;

namespace Moonwell
{
    /// <summary>What a package adds to the debug mode, in a development build: the objects only it knows, and its lines, drawings and statistics.</summary>
    internal interface IDebugView
    {
        /// <summary>Adds the objects this view knows within the radius of a point, other than units, which the session finds itself; an object already in the list is left out.</summary>
        void Candidates(float x, float y, float radius, List<Actor> into);

        /// <summary>Where an object this view knows stands; false for an object it does not know.</summary>
        bool Place(Actor actor, out float x, out float y);

        /// <summary>Adds this view's lines about the selected object to the panel.</summary>
        void Lines(Actor selected, DebugLines lines);

        /// <summary>Adds this view's drawings of the selected object.</summary>
        void Draw(Actor selected, DebugDraw draw);

        /// <summary>Adds this view's statistics to the panel.</summary>
        void Stats(DebugLines lines);
    }
}
#endif
