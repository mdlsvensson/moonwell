using Moonwell;
using static Moonwell.Objects.Human.Abilities;

namespace MoonwellMap
{
    /// <summary>A Storm Bolt with damage of its own, which tells the caster's owner whom it was thrown at.</summary>
    [Moonwell.Objects.ObjectId("A001")]
    public sealed class Smite : StormBolt
    {
        public Smite()
        {
            Name = "Smite";
            Damage = Levels(75f, 150f, 225f);
            Tooltip = PerLevel(level => $"Deals {Damage[level]} damage and stuns the target.");
        }

        public override void OnCast(Cast cast)
        {
            // A unit's name is in each player's own language, so it is read on the owner's machine only.
            Local.For(cast.Caster.Player, () =>
            {
                var target = cast.Target == null ? "nothing" : Natives.GetUnitName(cast.Target.Handle);
                Natives.DisplayTextToPlayer(cast.Caster.Player.Handle, 0f, 0f, "Smite, level " + cast.Level + ", at " + target);
            });
        }
    }
}
