export type EntityType = 'user' | 'org';

export interface BaseEntity {
  id: number;
  entity_type: EntityType;
  name: string;
  forge_id?: number;
}

export interface UserEntity extends BaseEntity {
  entity_type: 'user';
  email?: string;
  avatar?: string;
  admin?: boolean;
  description?: string;
  timezone?: string;
}

export interface OrgEntity extends BaseEntity {
  entity_type: 'org';
  is_user?: boolean;
  private?: boolean;
}

export type Entity = UserEntity | OrgEntity;

export interface EntityListOptions {
  page?: number;
  perPage?: number;
  entity_type?: string; // comma-separated: 'user', 'org', or 'user,org'
  is_admin?: boolean;
  search?: string;
}

// Type guards
export function isUserEntity(entity: Entity): entity is UserEntity {
  return entity.entity_type === 'user';
}

export function isOrgEntity(entity: Entity): entity is OrgEntity {
  return entity.entity_type === 'org';
}
