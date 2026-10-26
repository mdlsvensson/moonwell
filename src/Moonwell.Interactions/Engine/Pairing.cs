using System.Collections.Generic;

namespace Moonwell.Engine
{
    /// <summary>
    /// Makes and casts the pairs of an interaction, from outside the generic class. In Lua the body of
    /// Interaction&lt;THolder, TOther&gt; may not name its own Pair in a way CSharp.lua sets up while the classes are being
    /// defined (new Pair(), List&lt;Pair&gt;, a cast to Pair): the script would not load. So the interaction keeps its pairs as
    /// Owners, and makes and casts them here, where the class is named only when the call runs.
    /// </summary>
    internal static class Pairing
    {
        /// <summary>The pairs a check walks. One list is shared, as Find's candidates are: a walk adds the pairs at the end and takes them off again when it is done, so a check never walks a list that a reaction changes.</summary>
        private static readonly List<Owner> walk = new List<Owner>();

        /// <summary>The shared list of pairs being walked.</summary>
        public static List<Owner> Walk
        {
            get { return walk; }
        }

        /// <summary>A new pair of an interaction of a holder class and an other class, not yet joined to it.</summary>
        public static Interaction<H, O>.Pair Make<H, O>()
            where H : Actor
            where O : Actor
        {
            return new Interaction<H, O>.Pair();
        }

        // In Lua a pair is its own table and the cast checks nothing, so the call is left out.

        /// <summary>A pair kept as an Owner, as the pair it is.</summary>
        /// @CSharpLua.Template = "{0}"
        public static Interaction<H, O>.Pair As<H, O>(Owner pair)
            where H : Actor
            where O : Actor
        {
            return (Interaction<H, O>.Pair)pair;
        }

        /// <summary>Makes the read-only view of an interaction's pairs and keeps it on the interaction; the interaction's constructor calls it, so a read of Pairs makes nothing.</summary>
        public static void MakeView<H, O>(Interaction interaction)
            where H : Actor
            where O : Actor
        {
            interaction.view = new PairList<H, O>(interaction.pairs);
        }

        /// <summary>The read-only view of an interaction's pairs, made with the interaction.</summary>
        public static IReadOnlyList<Interaction<H, O>.Pair> View<H, O>(Interaction interaction)
            where H : Actor
            where O : Actor
        {
            return (PairList<H, O>)interaction.view!;
        }

        /// <summary>Takes the pairs a walk added off the end of the shared list, leaving those of the walks around it.</summary>
        public static void Release(int start)
        {
            Raw.Truncate(walk, start);
        }
    }
}
