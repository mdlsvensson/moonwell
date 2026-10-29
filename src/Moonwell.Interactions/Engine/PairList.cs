using System.Collections;
using System.Collections.Generic;

namespace Moonwell.Engine
{
    /// <summary>An interaction's pairs as a read-only list, in the order they met: a view of the list the interaction keeps, so it follows the pairs as they meet and part.</summary>
    internal sealed class PairList<H, O> : IReadOnlyList<Interaction<H, O>.Pair>
        where H : Actor
        where O : Actor
    {
        /// <summary>The interaction's pairs.</summary>
        private readonly List<Scope> pairs;

        /// <summary>Makes the view of an interaction's list of pairs.</summary>
        public PairList(List<Scope> pairs)
        {
            this.pairs = pairs;
        }

        /// <summary>How many pairs there are.</summary>
        public int Count
        {
            get { return pairs.Count; }
        }

        /// <summary>The pair at an index, in meeting order.</summary>
        public Interaction<H, O>.Pair this[int index]
        {
            get { return Pairing.As<H, O>(pairs[index]); }
        }

        /// <summary>The pairs in meeting order. A pair that parts while the loop runs moves those after it up, so a loop that may part pairs reads them by index from the end.</summary>
        public IEnumerator<Interaction<H, O>.Pair> GetEnumerator()
        {
            for (var i = 0; i < pairs.Count; i++)
            {
                yield return Pairing.As<H, O>(pairs[i]);
            }
        }

        /// <summary>The pairs in meeting order.</summary>
        IEnumerator IEnumerable.GetEnumerator()
        {
            return GetEnumerator();
        }
    }
}
