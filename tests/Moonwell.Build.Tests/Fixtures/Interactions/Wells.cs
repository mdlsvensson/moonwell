using Moonwell;
using Moonwell.Objects;

namespace InteractionsFixture
{
    /// <summary>Makes a footman and a well beside it at the start, and looks for the units near the well a second later.</summary>
    public sealed class Wells : GameComponent
    {
        public override void OnGameStart()
        {
            Unit.Create<Human.Units.Footman>(Player.One, 100f, 0f, Game.Instance);
            var well = Gizmo.Create(new Well(), 0f, 0f, Game.Instance, Player.One);
            After(1f, () =>
            {
                var near = Find.InRange<Unit>(well.X, well.Y, 500f);
                Game.Print("near the well: " + near.Count);
            });
        }
    }

    /// <summary>A map's own object with a place, which burns the units within range of it.</summary>
    public sealed class Well : Gizmo
    {
        protected override void OnCreated()
        {
            this.Interact(new FireAura(), this);
        }
    }

    /// <summary>Burns the enemies of the well's player within 300 of the well, every half second.</summary>
    public sealed class FireAura : Interaction<Well, Unit>
    {
        public FireAura()
        {
            Range = 300f;
            Tick = 0.5f;
        }

        public override bool Accepts(Unit other)
        {
            return Holder.Owner.IsEnemy(other.Owner);
        }

        public override void OnTick(Pair pair)
        {
            pair.Other.Life -= 10f;
        }
    }
}
