namespace Moonwell.Testing
{
    /// <summary>A missile the component cases fire, flying through the ground and units, that lists a Homing in its constructor, as a map's seeker does.</summary>
    public sealed class HomingDart : Missile
    {
        /// <summary>Makes a dart that flies through the ground and units with a Homing of a turn rate and no target.</summary>
        public HomingDart(float turnRate)
        {
            GroundContact = Contact.None;
            UnitContact = Contact.None;
            Components.Add(new Homing { TurnRate = turnRate });
        }
    }
}
