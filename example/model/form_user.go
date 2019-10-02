package model

// FormCreateUser 创建User，表单参数定义Name Age Department
// 参数校验： 1. 名称不能为空  2. 名称不能重复 (通过定义字段User)
type FormCreateUser struct {
	*User              `kitty:"bind:user.id,created_at,name,age;bindresult;"`
	Name               *string  `json:"-" kitty:"param:user.name;" vd:"len($)>0;msg:'name required'"`
	Age                *string  `json:"-" kitty:"param:user.age;"`
	Department         *string  `json:"-" kitty:"param:user.department;"`
	Salary             *float64 `json:"-" kitty:"param:user.salary;" vd:"$>0.0;msg:'salary is zero.'"`
	TestNameDumplicate *User    `json:"-" kitty:"getter:rd('name=name')|vf(this==nil?'name duplicate')"`
	MUser              User     `json:"-" kitty:"master"`
}

// FormUpdateUser 更新user.
type FormUpdateUser struct {
	ID                 *uint32 `kitty:"param:user.id;" vd:"$>0;msg:'please input id'"`
	Name               *string `json:"-" kitty:"param:user.name;"`
	Age                *string `json:"-" kitty:"param:user.age;"`
	Department         *string `json:"-" kitty:"param:user.department;"`
	TestID             *User   `json:"-" kitty:"getter:rd('id=id')|vf(this!=nil?'id not exist')"`
	TestNameDumplicate *User   `json:"-" kitty:"getter:rd('name=name')|vf(this==nil||this.id==id?'name duplicate')"`
	User               User    `json:"-" kitty:"master"`
}

// FormUser 用户信息/ 参数： ID/Name 两者其一
// 通过修改bind:user.*返回所有字段
type FormUser struct {
	*User `kitty:"bind:user.id,created_at,name,age;bindresult;"`
	ID    *uint32 `json:"-" kitty:"param:user.id;" vd:"$!=nil||(Name)$!=nil;msg:'id or name required.'"`
	Name  *string `json:"-" kitty:"param:user.name;"`
	MUser User    `json:"-" kitty:"master"`
}

// FormUserList 用户列表
// 查询条件: 创建时间 / 部门 .
// 规则：
type FormUserList struct {
	List       []*User `kitty:"bind:user.*;bindresult;"`
	Name       *string `json:"-" kitty:"param:user.name;"`
	CreateTime *string `json:"-" kitty:"param:user.created_at;"`
	Department *string `json:"-" kitty:"param:user.department;"`
	User       User    `json:"-" kitty:"master"`
}
